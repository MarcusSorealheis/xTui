package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/reflow/truncate"
)

func displayWidth(s string) int {
	return lipgloss.Width(s)
}

func cut(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if displayWidth(s) <= w {
		return s
	}
	var b strings.Builder
	n := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if rw < 1 {
			rw = 1
		}
		if n+rw > w {
			break
		}
		b.WriteRune(r)
		n += rw
	}
	return b.String()
}

func padPlain(s string, w int) string {
	s = cut(s, w)
	if gap := w - displayWidth(s); gap > 0 {
		s += strings.Repeat(" ", gap)
	}
	return s
}

func padWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = truncate.String(s, uint(w))
	if gap := w - lipgloss.Width(s); gap > 0 {
		s += strings.Repeat(" ", gap)
	}
	return s
}

func wrap(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	s = strings.ReplaceAll(s, "\t", " ")
	var out []string
	for _, para := range strings.Split(s, "\n") {
		if strings.TrimSpace(para) == "" {
			out = append(out, "")
			continue
		}
		out = append(out, wrapPara(para, width)...)
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

func wrapPara(para string, width int) []string {
	words := strings.Fields(para)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	cur := ""
	for _, word := range words {
		if displayWidth(word) > width {
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			for displayWidth(word) > width {
				lines = append(lines, cut(word, width))
				word = trimWidth(word, width)
			}
			cur = word
			continue
		}
		if cur == "" {
			cur = word
			continue
		}
		if displayWidth(cur)+1+displayWidth(word) <= width {
			cur += " " + word
			continue
		}
		lines = append(lines, cur)
		cur = word
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func trimWidth(s string, w int) string {
	n := 0
	for i, r := range s {
		rw := runewidth.RuneWidth(r)
		if rw < 1 {
			rw = 1
		}
		if n+rw > w {
			return s[i:]
		}
		n += rw
	}
	return ""
}

func relTime(t, now time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return t.Local().Format("Jan 2")
	}
}

func fit(s string, w, h int) string {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	for i, ln := range lines {
		lines[i] = padWidth(ln, w)
	}
	return strings.Join(lines, "\n")
}
