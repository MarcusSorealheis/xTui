package ui

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/MarcusSorealheis/xTui/internal/config"
	"github.com/MarcusSorealheis/xTui/internal/xapi"
)

func TestMain(m *testing.M) {
	copyText = func(string) error { return nil }
	openURL = func(string) error { return nil }
	os.Exit(m.Run())
}

func key(s string) tea.KeyMsg {
	special := map[string]tea.KeyType{
		"enter":     tea.KeyEnter,
		"esc":       tea.KeyEsc,
		"tab":       tea.KeyTab,
		"shift+tab": tea.KeyShiftTab,
		"up":        tea.KeyUp,
		"down":      tea.KeyDown,
		"pgup":      tea.KeyPgUp,
		"pgdown":    tea.KeyPgDown,
		"ctrl+c":    tea.KeyCtrlC,
		"ctrl+p":    tea.KeyCtrlP,
		"ctrl+d":    tea.KeyCtrlD,
		"ctrl+u":    tea.KeyCtrlU,
		"ctrl+x":    tea.KeyCtrlX,
	}
	if t, ok := special[s]; ok {
		return tea.KeyMsg{Type: t}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func boot(t *testing.T) Model {
	t.Helper()
	d := xapi.NewDemo()
	me, err := d.Me(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	m := New(Options{Service: d, Me: me, Demo: true})
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected initial load")
	}
	next, _ := m.Update(cmd())
	m = next.(Model)
	next, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return next.(Model)
}

func press(m Model, k string) (Model, tea.Cmd) {
	next, cmd := m.Update(key(k))
	return next.(Model), cmd
}

func settle(m Model, cmd tea.Cmd) Model {
	if cmd == nil {
		return m
	}
	msg := cmd()
	if _, quit := msg.(tea.QuitMsg); quit {
		return m
	}
	next, _ := m.Update(msg)
	return next.(Model)
}

var ansiRE = regexp.MustCompile(`\x1b(?:\[[0-9;]*[A-Za-z]|\][^\a]*(?:\a|\x1b\\))`)

func strip(s string) string { return ansiRE.ReplaceAllString(s, "") }

func TestKeysUniqueAndDispatchable(t *testing.T) {
	seen := map[string]string{}
	actions := map[string]bool{}
	for _, b := range bindings {
		actions[b.Action] = true
		for _, k := range b.Keys {
			if prev, ok := seen[k]; ok {
				t.Fatalf("duplicate key %q on %s and %s", k, prev, b.Action)
			}
			seen[k] = b.Action
		}
	}
	m := boot(t)
	for action := range actions {
		_, _, ok := m.dispatch(action)
		if !ok {
			t.Fatalf("dispatch missing %s", action)
		}
	}
}

func TestFeedRenderAndLike(t *testing.T) {
	m := boot(t)
	view := strip(m.View())
	for _, want := range []string{"xTui", "home", "@ada", "cache coherency", "like", "repost", "quote", "reply", "bookmark", "help", "quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q\n%s", want, view)
		}
	}
	if strings.Contains(view, "unlike") {
		t.Fatal("first post should not start liked")
	}
	if len(m.tweets) != 4 || m.tweets[0].Liked {
		t.Fatalf("tweets = %d liked %v", len(m.tweets), m.tweets[0].Liked)
	}
	m, cmd := press(m, "l")
	m = settle(m, cmd)
	if !m.tweets[0].Liked || m.tweets[0].Metrics.Likes != 129 {
		t.Fatalf("like state %+v", m.tweets[0].Metrics)
	}
	if !strings.Contains(strip(m.View()), "unlike") {
		t.Fatal(strip(m.View()))
	}
	lines := strings.Split(m.View(), "\n")
	if len(lines) != 40 {
		t.Fatalf("lines = %d", len(lines))
	}
}

func TestRepostConfirmReplyAndPage(t *testing.T) {
	m := boot(t)
	m, _ = press(m, "t")
	if m.mode != modeConfirm || !strings.Contains(strip(m.View()), "Repost") {
		t.Fatalf("mode %d view %s", m.mode, strip(m.View()))
	}
	m, _ = press(m, "n")
	if m.mode != modeMain || m.tweets[0].Reposted {
		t.Fatal("cancel should leave the post alone")
	}
	m, _ = press(m, "t")
	m, cmd := press(m, "y")
	m = settle(m, cmd)
	if !m.tweets[0].Reposted {
		t.Fatal("expected repost")
	}

	m, cmd = press(m, "r")
	if m.mode != modeCompose {
		t.Fatal("reply should open the composer")
	}
	for _, ch := range []string{"h", "i"} {
		m, cmd = press(m, ch)
		if cmd != nil {
			m = settle(m, cmd)
		}
	}
	if !strings.Contains(m.ta.Value(), "hi") {
		t.Fatalf("draft %q", m.ta.Value())
	}
	before := len(m.tweets)
	m, cmd = press(m, "ctrl+p")
	m = settle(m, cmd)
	if m.mode != modeMain || len(m.tweets) != before+1 || !strings.Contains(m.tweets[0].Text, "hi") {
		t.Fatalf("posted mode %d len %d text %q", m.mode, len(m.tweets), m.tweets[0].Text)
	}

	// The new post sits on top of a paged home timeline. Jump to the end and load the rest.
	m, cmd = press(m, "G")
	m = settle(m, cmd)
	if len(m.tweets) < 7 {
		t.Fatalf("paged len %d next %q", len(m.tweets), m.next)
	}
}

func TestHelpListsEveryAction(t *testing.T) {
	m := boot(t)
	m, _ = press(m, "?")
	view := strip(m.View())
	for _, b := range bindings {
		if !strings.Contains(view, b.Help) {
			t.Fatalf("help missing %q\n%s", b.Help, view)
		}
	}
	m, _ = press(m, "esc")
	if m.mode != modeMain {
		t.Fatal(m.mode)
	}
}

func TestSearchAndAuthorRoundTrip(t *testing.T) {
	m := boot(t)
	m, cmd := press(m, "/")
	m = settle(m, cmd)
	for _, ch := range strings.Split("cache", "") {
		if ch == "" {
			continue
		}
		m, cmd = press(m, ch)
		m = settle(m, cmd)
	}
	m, cmd = press(m, "enter")
	m = settle(m, cmd)
	if m.kind != feedSearch || len(m.tweets) == 0 || !strings.Contains(m.tweets[0].Text, "cache") {
		t.Fatalf("search %+v", m.tweets)
	}
	m, _ = press(m, "esc")
	if m.kind != feedHome {
		t.Fatalf("back kind %d", m.kind)
	}
}

func TestSetupScreen(t *testing.T) {
	t.Setenv("XTUI_CONFIG_DIR", t.TempDir())
	store := config.NewStore(config.Config{})
	m := New(Options{Store: store})
	m.width = 100
	m.height = 36
	if m.mode != modeEditID {
		t.Fatalf("mode %d", m.mode)
	}
	view := strip(m.View())
	if !strings.Contains(view, "127.0.0.1:53682/callback") || !strings.Contains(view, "enter") {
		t.Fatal(view)
	}
	m, _ = press(m, "a")
	m, _ = press(m, "b")
	m, cmd := press(m, "enter")
	if cmd == nil {
		t.Fatal("enter should start sign-in")
	}
	if m.mode != modeAuthWait {
		t.Fatalf("mode %d", m.mode)
	}
	if got := store.Get().ClientID; got != "ab" {
		t.Fatalf("client id %q", got)
	}
}

func TestLayoutFillsWidth(t *testing.T) {
	for _, width := range []int{80, 100, 120, 160} {
		lay := computeLayout(width, 40)
		if lay.sideW+lay.timeW+lay.detailW != width {
			t.Fatalf("width %d layout %+v", width, lay)
		}
	}
}

func TestRenderedLineWidths(t *testing.T) {
	m := boot(t)
	for _, sz := range [][2]int{{80, 24}, {100, 30}, {120, 40}, {160, 40}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		view := next.View()
		lines := strings.Split(view, "\n")
		if len(lines) != sz[1] {
			t.Fatalf("%dx%d lines=%d", sz[0], sz[1], len(lines))
		}
		for i, ln := range lines {
			if w := lipgloss.Width(ln); w != sz[0] {
				t.Errorf("%dx%d line %d width %d", sz[0], sz[1], i+1, w)
			}
		}
		if sz[0] == 120 {
			for i, ln := range strings.Split(strip(view), "\n") {
				if !strings.Contains(ln, "╭") && !strings.Contains(ln, "╰") && !strings.Contains(ln, "│") {
					continue
				}
				end := strings.TrimRight(ln, " ")
				if !strings.HasSuffix(end, "╮") && !strings.HasSuffix(end, "╯") && !strings.HasSuffix(end, "│") {
					t.Errorf("line %d border cut: %s", i+1, ln)
				}
			}
		}
	}
}

func TestNarrowAndQuit(t *testing.T) {
	m := boot(t)
	m.width = 70
	m.height = 24
	view := m.View()
	if strings.Count(view, "\n")+1 != 24 {
		t.Fatalf("height %d", strings.Count(view, "\n")+1)
	}
	_, cmd := press(m, "q")
	if cmd == nil {
		t.Fatal("quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected quit message")
	}
}
