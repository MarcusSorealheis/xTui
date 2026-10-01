package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/MarcusSorealheis/xTui/internal/xapi"
)

type paneLayout struct {
	sideW, timeW, detailW int
	showSide, showDetail  bool
	bodyH                 int
}

func computeLayout(width, height int) paneLayout {
	bodyH := height - 4
	if bodyH < 3 {
		bodyH = 3
	}
	lay := paneLayout{bodyH: bodyH}
	if width >= 100 {
		lay.showSide = true
		lay.sideW = 24
	}
	if width >= 92 {
		lay.showDetail = true
		lay.detailW = 38
	}
	lay.timeW = width - lay.sideW - lay.detailW
	if lay.timeW < 30 && lay.showDetail {
		lay.showDetail = false
		lay.detailW = 0
		lay.timeW = width - lay.sideW
	}
	if lay.timeW < 20 && lay.showSide {
		lay.showSide = false
		lay.sideW = 0
		lay.timeW = width
	}
	if lay.timeW < 1 {
		lay.timeW = 1
	}
	return lay
}

func (m Model) timelineBodyHeight() int {
	lay := computeLayout(m.width, m.height)
	inner := lay.bodyH - 2
	if inner < 1 {
		return 1
	}
	return inner
}

func (m Model) dialogWidth() int {
	w := m.width - 4
	if w > 72 {
		w = 72
	}
	if w < 28 {
		w = 28
	}
	if m.width > 4 && w > m.width-2 {
		w = m.width - 2
	}
	return w
}

// View draws the screen.
func (m Model) View() string {
	if m.width < 40 || m.height < 10 {
		return fit("xTui\nresize the terminal to at least 40×10", max(m.width, 20), max(m.height, 2))
	}
	header := m.renderHeader(m.width)
	footer := m.renderFooter(m.width)
	status := m.renderStatus(m.width)
	bodyH := m.height - lipgloss.Height(header) - lipgloss.Height(footer) - lipgloss.Height(status)
	if bodyH < 3 {
		bodyH = 3
	}
	var body string
	if m.mode == modeMain {
		body = m.renderPanels(bodyH)
	} else {
		body = lipgloss.Place(m.width, bodyH, lipgloss.Center, lipgloss.Center, m.dialog(bodyH))
	}
	return fit(lipgloss.JoinVertical(lipgloss.Left, header, body, status, footer), m.width, m.height)
}

func (m Model) renderHeader(w int) string {
	badge := lipgloss.NewStyle().Bold(true).Foreground(cBadgeText).Background(cAccent).Padding(0, 1).Render("xTui")
	feed := lipgloss.NewStyle().Bold(true).Foreground(cText).Render(m.label)
	left := badge + "  " + feed
	if m.me.Username != "" {
		left += "  " + lipgloss.NewStyle().Foreground(cMuted).Render(m.me.Handle())
	}
	if m.demo {
		left += "  " + lipgloss.NewStyle().Foreground(cYellow).Render("demo")
	}
	n := m.count()
	pos := "0/0"
	if n > 0 {
		pos = fmt.Sprintf("%d/%d", m.cursor+1, n)
	}
	right := pos
	if m.rate != "" {
		right += "   rl " + m.rate
	}
	rightR := lipgloss.NewStyle().Foreground(cMuted).Render(right)
	gap := w - lipgloss.Width(left) - lipgloss.Width(rightR)
	if gap < 1 {
		gap = 1
		left = cut(m.label, w/2)
	}
	line := left + strings.Repeat(" ", gap) + rightR
	return lipgloss.NewStyle().Background(cSurface).Width(w).Render(padWidth(line, w))
}

func (m Model) renderStatus(w int) string {
	text := m.status
	style := lipgloss.NewStyle().Foreground(cMuted)
	if m.statusErr {
		style = lipgloss.NewStyle().Foreground(cRed)
	}
	if text == "" {
		if tw, ok := m.selectedTweet(); ok {
			text = tw.URL()
		} else if m.mode == modeSetup {
			text = "sign in to read your home timeline"
		}
	}
	return style.Width(w).Render(padWidth(" "+text, w))
}

func (m Model) renderFooter(w int) string {
	var items []footItem
	switch m.mode {
	case modeHelp:
		items = []footItem{{"esc", "close"}, {"q", "close"}, {"ctrl+c", "quit"}}
	case modeConfirm:
		items = []footItem{{"y", m.confirm.yes}, {"n", "cancel"}, {"esc", "cancel"}}
	case modeCompose:
		items = []footItem{{"ctrl+p", "post"}, {"enter", "newline"}, {"esc", "cancel"}}
	case modeSearch:
		items = []footItem{{"enter", "search"}, {"esc", "cancel"}}
	case modeAuthWait:
		items = []footItem{{"esc", "cancel"}, {"o", "open again"}}
	case modeEditID:
		items = []footItem{{"enter", "continue"}, {"ctrl+s", "secret"}, {"esc", "back"}}
	case modeEditSecret:
		items = []footItem{{"enter", "save"}, {"esc", "back"}}
	case modeSetup:
		items = []footItem{{"enter", "sign in"}, {"i", "client id"}, {"s", "secret"}, {"d", "demo"}, {"q", "quit"}}
	default:
		items = mainFooter()
	}
	return footerBar(items, w)
}

func footerBar(items []footItem, width int) string {
	var rows []string
	var row string
	rowW := 0
	for _, item := range items {
		chip := styleKey().Render(item.keys) + " " + styleLabel().Render(item.label)
		cw := lipgloss.Width(chip)
		if row != "" && rowW+2+cw > width {
			rows = append(rows, row)
			row = ""
			rowW = 0
		}
		if row == "" {
			row = chip
			rowW = cw
			continue
		}
		row += "  " + chip
		rowW += 2 + cw
	}
	if row != "" {
		rows = append(rows, row)
	}
	for len(rows) < 2 {
		rows = append(rows, "")
	}
	if len(rows) > 2 {
		rows = rows[:2]
	}
	bar := lipgloss.NewStyle().Background(cCrust)
	return bar.Width(width).Render(padWidth(rows[0], width)) + "\n" + bar.Width(width).Render(padWidth(rows[1], width))
}

func (m Model) renderPanels(bodyH int) string {
	lay := computeLayout(m.width, m.height)
	lay.bodyH = bodyH
	var parts []string
	if lay.showSide {
		parts = append(parts, box("feeds", m.sideLines(lay.sideW-2, bodyH-2), lay.sideW, bodyH, m.focus == focusSide))
	}
	parts = append(parts, box(m.label, m.timelineLines(lay.timeW-2, bodyH-2), lay.timeW, bodyH, m.focus == focusTimeline))
	if lay.showDetail {
		title := "post"
		if m.threadID != "" {
			title = "thread"
		}
		if m.kind == feedLists {
			title = "list"
		}
		parts = append(parts, box(title, m.detailLines(lay.detailW-2, bodyH-2), lay.detailW, bodyH, m.focus == focusDetail))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func box(title string, body []string, width, height int, active bool) string {
	if width < 4 {
		width = 4
	}
	if height < 3 {
		height = 3
	}
	innerW := width - 2
	innerH := height - 2
	for len(body) < innerH {
		body = append(body, "")
	}
	if len(body) > innerH {
		body = body[:innerH]
	}
	color := cBorder
	if active {
		color = cAccent
	}
	edge := lipgloss.NewStyle().Foreground(color)
	label := cut(" "+title+" ", innerW-2)
	if innerW < 4 {
		label = cut(title, innerW)
	}
	left := 1
	if left+displayWidth(label) > innerW {
		left = 0
	}
	right := innerW - left - displayWidth(label)
	if right < 0 {
		right = 0
	}
	top := edge.Render("╭"+strings.Repeat("─", left)) + lipgloss.NewStyle().Bold(true).Foreground(cAccent).Render(label) + edge.Render(strings.Repeat("─", right)+"╮")
	bot := edge.Render("╰" + strings.Repeat("─", innerW) + "╯")
	var b strings.Builder
	b.WriteString(top)
	for _, ln := range body {
		b.WriteByte('\n')
		b.WriteString(edge.Render("│"))
		b.WriteString(padWidth(ln, innerW))
		b.WriteString(edge.Render("│"))
	}
	b.WriteByte('\n')
	b.WriteString(bot)
	return b.String()
}

func (m Model) sideLines(w, h int) []string {
	if w < 1 {
		w = 1
	}
	var lines []string
	for i, sf := range sideFeeds {
		marker := "  "
		style := lipgloss.NewStyle().Foreground(cText)
		if i == m.side {
			marker = "▸ "
			style = style.Bold(true)
		}
		key := styleKey().Render(sf.key)
		label := style.Render(sf.label)
		lines = append(lines, padWidth(" "+marker+key+" "+label, w))
	}
	lines = append(lines, "")
	lines = append(lines, padWidth(styleLabel().Render(" tab pane"), w))
	lines = append(lines, padWidth(styleLabel().Render(" 1-6 feeds"), w))
	if m.me.Description != "" {
		lines = append(lines, "")
		for _, ln := range wrap(m.me.Description, w-1) {
			lines = append(lines, padWidth(styleLabel().Render(" "+ln), w))
		}
	}
	return fitLines(lines, w, h)
}

func (m Model) timelineLines(w, h int) []string {
	if w < 1 {
		w = 1
	}
	if m.kind == feedLists {
		return m.listLines(w, h)
	}
	if len(m.tweets) == 0 {
		msg := "no posts"
		if m.loading {
			msg = "loading " + m.label + "…"
		} else if m.statusErr && m.status != "" {
			msg = m.status
		}
		return fitLines(wrap(msg, w), w, h)
	}
	var lines []string
	vis := h / 3
	if vis < 1 {
		vis = 1
	}
	end := m.offset + vis
	if end > len(m.tweets) {
		end = len(m.tweets)
	}
	start := m.offset
	if start < 0 {
		start = 0
	}
	if m.threadID != "" && !computeLayout(m.width, m.height).showDetail {
		return fitLines(m.threadBlock(w), w, h)
	}
	for i := start; i < end; i++ {
		lines = append(lines, m.tweetRow(m.tweets[i], w, i == m.cursor)...)
	}
	if m.loading && m.next != "" {
		lines = append(lines, styleLabel().Render(" loading more…"))
	}
	return fitLines(lines, w, h)
}

func (m Model) listLines(w, h int) []string {
	if len(m.lists) == 0 {
		msg := "no lists"
		if m.loading {
			msg = "loading lists…"
		}
		return fitLines([]string{msg}, w, h)
	}
	var lines []string
	vis := h / 2
	if vis < 1 {
		vis = 1
	}
	end := m.offset + vis
	if end > len(m.lists) {
		end = len(m.lists)
	}
	for i := m.offset; i < end; i++ {
		if i < 0 {
			continue
		}
		list := m.lists[i]
		name := list.Name
		if name == "" {
			name = list.ID
		}
		meta := fmt.Sprintf("%d members", list.Members)
		if list.Owner != "" {
			meta = "@" + list.Owner + "  " + meta
		}
		row1 := name
		row2 := meta
		if list.Description != "" {
			row2 = cut(list.Description, w)
		}
		if i == m.cursor {
			row1 = lipgloss.NewStyle().Bold(true).Background(cSurface).Foreground(cText).Width(w).Render(cut("▸ "+row1, w))
			row2 = lipgloss.NewStyle().Background(cSurface).Foreground(cMuted).Width(w).Render(cut("  "+row2, w))
		} else {
			row1 = padPlain("  "+row1, w)
			row2 = styleLabel().Width(w).Render(cut("  "+row2, w))
		}
		lines = append(lines, row1, row2)
	}
	return fitLines(lines, w, h)
}

func (m Model) tweetRow(t xapi.Tweet, w int, selected bool) []string {
	who := t.Author.DisplayName()
	if t.Author.Verified {
		who += " ✓"
	}
	if t.Author.Username != "" {
		who += "  " + t.Author.Handle()
	}
	when := relTime(t.CreatedAt, m.clock())
	prefix := "  "
	if selected {
		prefix = "▸ "
	}
	left := prefix + who
	gap := w - displayWidth(left) - displayWidth(when)
	if gap < 1 {
		left = cut(left, w-displayWidth(when)-1)
		gap = 1
	}
	line1 := left + strings.Repeat(" ", gap) + when
	text := strings.Join(strings.Fields(t.Text), " ")
	if t.IsRepost {
		text = "reposted by " + t.RepostedBy.Handle() + "  " + text
	}
	line2 := "  " + text
	metrics := m.metrics(t)
	context := rowContext(t)
	line3 := "  " + metrics
	if context != "" {
		line3 += "   " + context
	}
	bg := lipgloss.NewStyle()
	if selected {
		bg = bg.Background(cSurface)
	}
	return []string{
		bg.Width(w).Render(padWidth(line1, w)),
		bg.Width(w).Foreground(cText).Render(padWidth(cut(line2, w), w)),
		bg.Width(w).Render(padWidth(cutPlainMetrics(line3, w), w)),
	}
}

func cutPlainMetrics(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	return truncateANSI(s, w)
}

func truncateANSI(s string, w int) string {
	return padWidth(s, w)
}

func (m Model) metrics(t xapi.Tweet) string {
	heart := lipgloss.NewStyle().Foreground(cMuted).Render("♥ " + fmtCount(t.Metrics.Likes))
	if t.Liked {
		heart = lipgloss.NewStyle().Foreground(cPink).Render("♥ " + fmtCount(t.Metrics.Likes))
	}
	repo := lipgloss.NewStyle().Foreground(cMuted).Render("↻ " + fmtCount(t.Metrics.Reposts))
	if t.Reposted {
		repo = lipgloss.NewStyle().Foreground(cGreen).Render("↻ " + fmtCount(t.Metrics.Reposts))
	}
	talk := lipgloss.NewStyle().Foreground(cMuted).Render("💬 " + fmtCount(t.Metrics.Replies))
	parts := heart + "  " + repo + "  " + talk
	if t.Bookmarked {
		parts += "  " + lipgloss.NewStyle().Foreground(cYellow).Render("★")
	}
	return parts
}

func rowContext(t xapi.Tweet) string {
	var bits []string
	if to := t.ReplyTo(); to != "" {
		bits = append(bits, "reply @"+to)
	}
	if q, ok := t.Quote(); ok {
		who := q.Author
		if who == "" {
			who = "post"
		}
		bits = append(bits, "quote @"+who)
	}
	if len(t.Media) > 0 {
		kind := t.Media[0].Type
		if kind == "" {
			kind = "media"
		}
		bits = append(bits, kind)
	}
	if t.Hidden {
		bits = append(bits, "hidden")
	}
	return strings.Join(bits, "  ")
}

func (m Model) detailLines(w, h int) []string {
	if w < 1 {
		w = 1
	}
	lines := m.detailBlock(w)
	off := m.detailOff
	if off < 0 {
		off = 0
	}
	if off > len(lines) {
		off = len(lines)
	}
	if off > 0 {
		lines = lines[off:]
	}
	return fitLines(lines, w, h)
}

func (m Model) detailBlock(w int) []string {
	if m.kind == feedLists {
		list, ok := m.selectedList()
		if !ok {
			return []string{"select a list", "", "enter opens it"}
		}
		lines := []string{list.Name, ""}
		if list.Owner != "" {
			lines = append(lines, "@"+list.Owner)
		}
		lines = append(lines, fmt.Sprintf("%d members", list.Members), "")
		lines = append(lines, wrap(list.Description, w)...)
		lines = append(lines, "", "enter  open")
		return lines
	}
	if m.threadID != "" && len(m.thread) > 0 {
		var lines []string
		for i, t := range m.thread {
			if i > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, m.tweetBlock(t, w, i == 0)...)
		}
		return lines
	}
	tw, ok := m.selectedTweet()
	if !ok {
		return []string{"select a post"}
	}
	lines := m.tweetBlock(tw, w, true)
	lines = append(lines, "", styleKey().Render("actions"))
	lines = append(lines, m.actionLines(tw, w)...)
	return lines
}

func (m Model) threadBlock(w int) []string {
	if len(m.thread) == 0 {
		tw, ok := m.selectedTweet()
		if !ok {
			return []string{"loading thread…"}
		}
		return m.tweetBlock(tw, w, true)
	}
	return m.detailBlock(w)
}

func (m Model) tweetBlock(t xapi.Tweet, w int, menu bool) []string {
	head := t.Author.DisplayName()
	if t.Author.Verified {
		head += " ✓"
	}
	if t.Author.Username != "" {
		head += "  " + t.Author.Handle()
	}
	lines := []string{head, relTime(t.CreatedAt, m.clock())}
	if t.IsRepost {
		lines = append(lines, "reposted by "+t.RepostedBy.Handle())
	}
	if to := t.ReplyTo(); to != "" {
		lines = append(lines, "replying to @"+to)
	}
	lines = append(lines, "")
	lines = append(lines, wrap(t.Text, w)...)
	if q, ok := t.Quote(); ok {
		lines = append(lines, "")
		quote := "quote"
		if q.Author != "" {
			quote += " @" + q.Author
		}
		lines = append(lines, styleLabel().Render(quote))
		if q.Text != "" {
			for _, ln := range wrap(q.Text, w-2) {
				lines = append(lines, styleLabel().Render("│ "+ln))
			}
		}
	}
	for _, media := range t.Media {
		label := media.Type
		if label == "" {
			label = "media"
		}
		if media.Alt != "" {
			label += " · " + media.Alt
		}
		lines = append(lines, styleLabel().Render(label))
	}
	if t.Sensitive {
		lines = append(lines, styleLabel().Render("sensitive"))
	}
	if t.Hidden {
		lines = append(lines, styleLabel().Render("hidden reply"))
	}
	lines = append(lines, "", m.metrics(t), t.URL())
	if menu {
		return lines
	}
	return lines
}

func (m Model) actionLines(t xapi.Tweet, w int) []string {
	like, repost, mark := "like", "repost", "bookmark"
	if t.Liked {
		like = "unlike"
	}
	if t.Reposted {
		repost = "undo repost"
	}
	if t.Bookmarked {
		mark = "remove bookmark"
	}
	follow, mute, block := "follow", "mute", "block"
	if t.Author.Following {
		follow = "unfollow"
	}
	if t.Author.Muting {
		mute = "unmute"
	}
	if t.Author.Blocking {
		block = "unblock"
	}
	rows := []struct{ key, label string }{
		{"l", like},
		{"t", repost},
		{"T", "quote"},
		{"r", "reply"},
		{"b", mark},
		{"f", follow},
		{"m", mute},
		{"B", block},
		{"h", "hide reply"},
		{"d", "delete"},
		{"c", "copy link"},
		{"o", "open in browser"},
		{"u", "author posts"},
		{"n", "new post"},
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		line := styleKey().Render(padPlain(row.key, 1)) + "  " + row.label
		out = append(out, padWidth(line, w))
	}
	return out
}

func (m Model) dialog(maxH int) string {
	w := m.dialogWidth()
	inner := w - 4
	if inner < 10 {
		inner = 10
	}
	var title, body string
	switch m.mode {
	case modeHelp:
		title = "keys"
		body = m.helpBody(inner, maxH-6)
	case modeConfirm:
		title = m.confirm.title
		body = wrapJoin(m.confirm.body, inner) + "\n\ny  " + m.confirm.yes + "      n  cancel"
	case modeCompose:
		title = "new post"
		if m.composeKind == "reply" {
			title = "reply to " + m.composeWho
		} else if m.composeKind == "quote" {
			title = "quote " + m.composeWho
		}
		n := len([]rune(m.ta.Value()))
		body = m.ta.View() + "\n\n" + fmt.Sprintf("%d characters    ctrl+p posts    esc cancels", n)
	case modeSearch:
		title = "search"
		body = m.search.View() + "\n\nrecent posts, from:handle, or a phrase"
	case modeAuthWait:
		title = "sign in"
		waited := m.authTicks
		if waited < 1 {
			waited = 1
		}
		lines := []string{
			"Approve xTui in the browser.",
			fmt.Sprintf("Waiting %ds. esc cancels.", waited),
		}
		if m.authURL != "" {
			lines = append(lines, "", m.authURL)
		}
		body = wrapJoin(strings.Join(lines, "\n"), inner)
	case modeEditID:
		_, _, redirect := m.currentConfig()
		title = "sign in"
		body = m.clientID.View() + "\n\n" + wrapJoin("Paste the client id, then press enter.\nCallback "+redirect, inner)
	case modeEditSecret:
		title = "client secret"
		body = m.secret.View() + "\n\nLeave this empty for a public app.\nenter saves"
	default:
		title = "xTui"
		body = m.setupBody(inner)
	}
	content := lipgloss.NewStyle().Bold(true).Foreground(cAccent).Render(title) + "\n\n" + body
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cAccent).
		Foreground(cText).
		Padding(1, 2).
		Width(inner).
		Render(content)
	if lipgloss.Height(box) > maxH && maxH > 2 {
		lines := strings.Split(box, "\n")
		if len(lines) > maxH {
			lines = lines[:maxH]
		}
		box = strings.Join(lines, "\n")
	}
	return box
}

func (m Model) setupBody(w int) string {
	id, secret, redirect := m.currentConfig()
	if id == "" {
		id = "not set"
	}
	sec := "not set"
	if secret != "" {
		sec = "saved"
	}
	text := strings.Join([]string{
		"Enter opens the browser.",
		"",
		"client id  " + id,
		"secret     " + sec,
		"callback   " + redirect,
	}, "\n")
	return wrapJoin(text, w)
}

func (m Model) helpBody(w, maxLines int) string {
	if maxLines < 8 {
		maxLines = 8
	}
	type row struct{ keys, help string }
	var rows []row
	seen := map[string]bool{}
	for _, b := range bindings {
		if seen[b.Action] {
			continue
		}
		seen[b.Action] = true
		keys := strings.Join(b.Keys, ", ")
		rows = append(rows, row{keys: keys, help: b.Help})
	}
	col := w / 2
	if col < 20 {
		col = w
	}
	var lines []string
	if col == w {
		for _, row := range rows {
			lines = append(lines, cut(row.keys+"  "+row.help, w))
		}
	} else {
		mid := (len(rows) + 1) / 2
		for i := 0; i < mid; i++ {
			left := cut(fmt.Sprintf("%-14s %s", cut(rows[i].keys, 14), rows[i].help), col-1)
			right := ""
			if j := i + mid; j < len(rows) {
				right = cut(fmt.Sprintf("%-14s %s", cut(rows[j].keys, 14), rows[j].help), col-1)
			}
			lines = append(lines, padPlain(left, col)+right)
		}
	}
	lines = append(lines, "")
	lines = append(lines, cut("y confirms. Like and bookmark toggle. ctrl+p posts.", w))
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n")
}

func wrapJoin(s string, w int) string {
	var out []string
	for _, para := range strings.Split(s, "\n") {
		if strings.TrimSpace(para) == "" {
			out = append(out, "")
			continue
		}
		out = append(out, wrap(para, w)...)
	}
	return strings.Join(out, "\n")
}

func fitLines(lines []string, w, h int) []string {
	if h < 0 {
		h = 0
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	out := make([]string, h)
	for i := 0; i < h; i++ {
		ln := ""
		if i < len(lines) {
			ln = lines[i]
		}
		out[i] = padWidth(ln, w)
	}
	return out
}

// The setup screen reads the store directly.
func (m Model) currentConfig() (id, secret, redirect string) {
	redirect = "http://127.0.0.1:53682/callback"
	if m.store == nil {
		return "", "", redirect
	}
	cfg := m.store.Get()
	return cfg.ClientID, cfg.ClientSecret, cfg.RedirectURI()
}
