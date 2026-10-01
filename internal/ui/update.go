package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/MarcusSorealheis/xTui/internal/config"
	"github.com/MarcusSorealheis/xTui/internal/xapi"
)

// Update handles keys, feed pages, and post actions.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.onKey(msg)
	case tea.MouseMsg:
		return m.onMouse(msg)
	case authURLMsg:
		if m.mode == modeAuthWait {
			m.authURL = msg.url
		}
		return m, nil
	case authTickMsg:
		if m.mode != modeAuthWait {
			return m, nil
		}
		m.authTicks++
		return m, authTick()
	case startSignInMsg:
		return m.beginAuth()
	case feedLoaded:
		return m.onFeed(msg), nil
	case listsLoaded:
		return m.onLists(msg), nil
	case threadLoaded:
		return m.onThread(msg), nil
	case actionResult:
		return m.onAction(msg), nil
	case notice:
		m.status = msg.text
		m.statusErr = msg.isErr
		return m, nil
	case authDone:
		return m.onAuth(msg)
	default:
		return m, nil
	}
}

func (m Model) onKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	if s == "ctrl+c" {
		return m, quitCmd()
	}
	switch m.mode {
	case modeHelp:
		if s == "esc" || s == "q" || s == "?" {
			m.mode = m.helpFrom
			if m.mode == modeHelp {
				m.mode = modeMain
			}
			return m, nil
		}
		return m, nil
	case modeConfirm:
		switch s {
		case "y", "Y":
			return m.runConfirm()
		case "n", "N", "esc", "q":
			m.mode = modeMain
			m.status = ""
			return m, nil
		}
		return m, nil
	case modeCompose:
		switch s {
		case "esc":
			m.mode = modeMain
			m.ta.Blur()
			return m, nil
		case "ctrl+p":
			return m.submitCompose()
		default:
			var cmd tea.Cmd
			m.ta, cmd = m.ta.Update(k)
			return m, cmd
		}
	case modeSearch:
		switch s {
		case "esc":
			m.mode = modeMain
			m.search.Blur()
			return m, nil
		case "enter":
			return m.submitSearch()
		default:
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(k)
			return m, cmd
		}
	case modeAuthWait:
		switch s {
		case "esc", "q":
			if m.authCancel != nil {
				m.authCancel()
			}
			m.mode = modeSetup
			m.authURL = ""
			m.authTicks = 0
			m.status = "sign-in cancelled"
			m.statusErr = false
			return m, nil
		case "o":
			if m.authURL == "" {
				m.status = "still preparing the sign-in link"
				return m, nil
			}
			target := m.authURL
			return m, func() tea.Msg {
				if err := openURL(target); err != nil {
					return notice{text: err.Error(), isErr: true}
				}
				return notice{text: "opened the sign-in page again", isErr: false}
			}
		}
		m.status = "waiting for the browser. esc cancels, o opens the page again"
		m.statusErr = false
		return m, nil
	case modeEditID, modeEditSecret:
		switch s {
		case "esc":
			m.mode = modeSetup
			m.clientID.Blur()
			m.secret.Blur()
			return m, nil
		case "ctrl+s":
			if m.mode == modeEditID {
				return m.editSecret()
			}
			return m, nil
		case "enter":
			if m.mode == modeEditID {
				return m.saveClientID()
			}
			return m.saveSecret()
		default:
			var cmd tea.Cmd
			if m.mode == modeEditID {
				m.clientID, cmd = m.clientID.Update(k)
			} else {
				m.secret, cmd = m.secret.Update(k)
			}
			return m, cmd
		}
	case modeSetup:
		switch s {
		case "a", "enter":
			return m.beginAuth()
		case "i":
			return m.editClientID()
		case "s":
			return m.editSecret()
		case "d":
			return m.enterDemo()
		case "q":
			return m, quitCmd()
		case "?":
			next, cmd, _ := m.openHelp()
			return next, cmd
		}
		return m, nil
	default:
		if act, ok := actionFor(s); ok {
			next, cmd, _ := m.dispatch(act)
			return next, cmd
		}
	}
	return m, nil
}

func (m Model) dispatch(action string) (Model, tea.Cmd, bool) {
	switch action {
	case actNext:
		return m.moveFocus(1)
	case actPrev:
		return m.moveFocus(-1)
	case actTop:
		m.threadID, m.thread = "", nil
		m.cursor, m.offset, m.detailOff = 0, 0, 0
		return m, nil, true
	case actBottom:
		m.threadID, m.thread = "", nil
		m.detailOff = 0
		if n := m.count(); n > 0 {
			m.cursor = n - 1
			m = m.reveal()
		}
		next, cmd, _ := m.maybePage()
		return next, cmd, true
	case actPageDown:
		if m.focus == focusDetail {
			m.detailOff += m.detailPage()
			if m.detailOff < 0 {
				m.detailOff = 0
			}
			return m, nil, true
		}
		return m.move(m.visible())
	case actPageUp:
		if m.focus == focusDetail {
			m.detailOff -= m.detailPage()
			if m.detailOff < 0 {
				m.detailOff = 0
			}
			return m, nil, true
		}
		return m.move(-m.visible())
	case actPaneNext:
		m.focus = m.cycle(1)
		return m, nil, true
	case actPanePrev:
		m.focus = m.cycle(-1)
		return m, nil, true
	case actHome:
		return m.openSide(0)
	case actMentions:
		return m.openSide(1)
	case actMarks:
		return m.openSide(2)
	case actLikes:
		return m.openSide(3)
	case actPosts:
		return m.openSide(4)
	case actLists:
		return m.openSide(5)
	case actOpen:
		if m.kind == feedLists {
			return m.openList()
		}
		return m.openThread()
	case actBack:
		return m.back()
	case actRefresh:
		return m.refresh()
	case actLike:
		return m.toggleLike()
	case actRepost:
		return m.ask("repost")
	case actQuote:
		return m.openCompose("quote")
	case actReply:
		return m.openCompose("reply")
	case actBookmark:
		return m.toggleBookmark()
	case actFollow:
		return m.ask("follow")
	case actMute:
		return m.ask("mute")
	case actBlock:
		return m.ask("block")
	case actHide:
		return m.ask("hide")
	case actDelete:
		return m.ask("delete")
	case actCopy:
		return m.copyLink()
	case actBrowser:
		return m.openBrowser()
	case actAuthor:
		return m.openAuthor()
	case actCompose:
		return m.openCompose("post")
	case actSearch:
		return m.startSearch()
	case actHelp:
		return m.openHelp()
	case actQuit:
		return m, quitCmd(), true
	case actSignOut:
		return m.signOut()
	default:
		return m, nil, false
	}
}

func (m Model) moveFocus(delta int) (Model, tea.Cmd, bool) {
	lay := computeLayout(m.width, m.height)
	if m.focus == focusDetail && lay.showDetail {
		m.detailOff += delta
		if m.detailOff < 0 {
			m.detailOff = 0
		}
		return m, nil, true
	}
	if m.focus == focusSide && lay.showSide {
		return m.moveSide(delta)
	}
	return m.move(delta)
}

func (m Model) move(delta int) (Model, tea.Cmd, bool) {
	if m.count() == 0 {
		return m, nil, true
	}
	m.threadID = ""
	m.thread = nil
	m.detailOff = 0
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= m.count() {
		m.cursor = m.count() - 1
	}
	m = m.reveal()
	next, cmd, _ := m.maybePage()
	return next, cmd, true
}

func (m Model) maybePage() (Model, tea.Cmd, bool) {
	if m.kind != feedLists && m.next != "" && !m.loading && m.cursor >= len(m.tweets)-1 {
		next, cmd := m.reload(true)
		return next, cmd, true
	}
	return m, nil, true
}

func (m Model) moveSide(delta int) (Model, tea.Cmd, bool) {
	n := len(sideFeeds)
	next := m.side + delta
	if m.side < 0 {
		next = 0
	}
	if next < 0 {
		next = 0
	}
	if next >= n {
		next = n - 1
	}
	return m.openSide(next)
}

func (m Model) cycle(delta int) focus {
	panes := m.panes()
	if len(panes) == 0 {
		return focusTimeline
	}
	cur := m.focus
	idx := 0
	for i, p := range panes {
		if p == cur {
			idx = i
			break
		}
	}
	idx = (idx + delta) % len(panes)
	if idx < 0 {
		idx += len(panes)
	}
	return panes[idx]
}

func (m Model) panes() []focus {
	lay := computeLayout(m.width, m.height)
	var p []focus
	if lay.showSide {
		p = append(p, focusSide)
	}
	p = append(p, focusTimeline)
	if lay.showDetail {
		p = append(p, focusDetail)
	}
	return p
}

func (m Model) openSide(i int) (Model, tea.Cmd, bool) {
	if i < 0 || i >= len(sideFeeds) {
		return m, nil, true
	}
	sf := sideFeeds[i]
	arg := ""
	if sf.kind == feedUser {
		arg = m.me.ID
	}
	if m.kind == sf.kind && m.arg == arg && m.side == i && len(m.stack) == 0 {
		m.side = i
		return m, nil, true
	}
	m.stack = nil
	m.side = i
	m.kind = sf.kind
	m.arg = arg
	m.label = sf.label
	m.cursor, m.offset = 0, 0
	m.tweets, m.lists = nil, nil
	m.next = ""
	m.thread, m.threadID = nil, ""
	m.detailOff = 0
	m.mode = modeMain
	next, cmd := m.reload(false)
	return next, cmd, true
}

func (m Model) openList() (Model, tea.Cmd, bool) {
	list, ok := m.selectedList()
	if !ok {
		m.status = "no list selected"
		m.statusErr = true
		return m, nil, true
	}
	m = m.push()
	m.kind = feedListTweets
	m.arg = list.ID
	m.label = "list · " + list.Name
	m.tweets = nil
	m.cursor, m.offset = 0, 0
	m.side = -1
	m.thread, m.threadID = nil, ""
	next, cmd := m.reload(false)
	return next, cmd, true
}

func (m Model) openThread() (Model, tea.Cmd, bool) {
	tw, ok := m.selectedTweet()
	if !ok {
		m.status = "no post selected"
		m.statusErr = true
		return m, nil, true
	}
	m.threadID = tw.ID
	m.thread = nil
	m.detailOff = 0
	if !computeLayout(m.width, m.height).showDetail {
		m.focus = focusTimeline
	} else {
		m.focus = focusDetail
	}
	m.status = "loading thread…"
	m.statusErr = false
	svc := m.svc
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		tweets, err := svc.Thread(ctx, tw)
		return threadLoaded{id: tw.ID, tweets: tweets, err: err}
	}, true
}

func (m Model) openAuthor() (Model, tea.Cmd, bool) {
	tw, ok := m.selectedTweet()
	if !ok || tw.Author.ID == "" {
		m.status = "no author on this post"
		m.statusErr = true
		return m, nil, true
	}
	m = m.push()
	m.kind = feedUser
	m.arg = tw.Author.ID
	m.label = "posts · " + tw.Author.Handle()
	m.side = -1
	m.tweets = nil
	m.cursor, m.offset = 0, 0
	m.thread, m.threadID = nil, ""
	next, cmd := m.reload(false)
	return next, cmd, true
}

func (m Model) back() (Model, tea.Cmd, bool) {
	if m.threadID != "" {
		m.threadID = ""
		m.thread = nil
		m.detailOff = 0
		m.focus = focusTimeline
		return m, nil, true
	}
	if next, ok := m.pop(); ok {
		next.status = ""
		return next, nil, true
	}
	m.status = ""
	return m, nil, true
}

func (m Model) refresh() (Model, tea.Cmd, bool) {
	next, cmd := m.reload(false)
	return next, cmd, true
}

func (m Model) openHelp() (Model, tea.Cmd, bool) {
	m.helpFrom = m.mode
	if m.helpFrom == modeHelp {
		m.helpFrom = modeMain
	}
	m.mode = modeHelp
	return m, nil, true
}

func (m Model) signOut() (Model, tea.Cmd, bool) {
	if m.store != nil {
		_ = m.store.Update(func(c *config.Config) {
			*c = config.ClearTokens(*c)
		})
	}
	m.svc = nil
	m.me = xapi.User{}
	m.demo = false
	m.tweets = nil
	m.lists = nil
	m.stack = nil
	m.mode = modeSetup
	m.status = "signed out"
	m.statusErr = false
	return m, nil, true
}

func (m Model) needTweet() (Model, xapi.Tweet, bool) {
	tw, ok := m.selectedTweet()
	if ok {
		return m, tw, true
	}
	m.status = "no post selected"
	m.statusErr = true
	return m, xapi.Tweet{}, false
}

func (m Model) toggleLike() (Model, tea.Cmd, bool) {
	m, tw, ok := m.needTweet()
	if !ok {
		return m, nil, true
	}
	want := !tw.Liked
	m = m.applyLike(tw.ID, tw.ActionID(), want)
	if want {
		m.status = "liked"
	} else {
		m.status = "unliked"
	}
	m.statusErr = false
	return m.act("like", tw, want, func(ctx context.Context, id string) error {
		if want {
			return m.svc.Like(ctx, id)
		}
		return m.svc.Unlike(ctx, id)
	})
}

func (m Model) toggleBookmark() (Model, tea.Cmd, bool) {
	m, tw, ok := m.needTweet()
	if !ok {
		return m, nil, true
	}
	want := !tw.Bookmarked
	m = m.walk(tw.ID, tw.ActionID(), func(t *xapi.Tweet) { t.Bookmarked = want })
	if want {
		m.status = "bookmarked"
	} else {
		m.status = "bookmark removed"
	}
	m.statusErr = false
	return m.act("bookmark", tw, want, func(ctx context.Context, id string) error {
		if want {
			return m.svc.Bookmark(ctx, id)
		}
		return m.svc.Unbookmark(ctx, id)
	})
}

func (m Model) act(op string, tw xapi.Tweet, want bool, fn func(context.Context, string) error) (Model, tea.Cmd, bool) {
	var seq int
	m, seq = m.bump(op, tw.ID)
	id := tw.ActionID()
	row := tw.ID
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		err := fn(ctx, id)
		return actionResult{op: op, rowID: row, actionID: id, seq: seq, want: want, err: err}
	}, true
}

func (m Model) ask(action string) (Model, tea.Cmd, bool) {
	m, tw, ok := m.needTweet()
	if !ok {
		return m, nil, true
	}
	c := confirmState{action: action, tweet: tw}
	switch action {
	case "repost":
		c.want = !tw.Reposted
		if c.want {
			c.title, c.yes = "Repost", "repost"
		} else {
			c.title, c.yes = "Undo repost", "undo"
		}
		c.body = preview(tw)
	case "follow":
		c.want = !tw.Author.Following
		if c.want {
			c.title, c.yes = "Follow "+tw.Author.Handle(), "follow"
		} else {
			c.title, c.yes = "Unfollow "+tw.Author.Handle(), "unfollow"
		}
		c.body = tw.Author.DisplayName()
	case "mute":
		c.want = !tw.Author.Muting
		if c.want {
			c.title, c.yes = "Mute "+tw.Author.Handle(), "mute"
		} else {
			c.title, c.yes = "Unmute "+tw.Author.Handle(), "unmute"
		}
		c.body = "You will stop seeing their posts."
	case "block":
		c.want = !tw.Author.Blocking
		if c.want {
			c.title, c.yes = "Block "+tw.Author.Handle(), "block"
		} else {
			c.title, c.yes = "Unblock "+tw.Author.Handle(), "unblock"
		}
		c.body = "They will not be able to follow you or see your posts."
	case "hide":
		c.want = !tw.Hidden
		if c.want {
			c.title, c.yes = "Hide reply", "hide"
		} else {
			c.title, c.yes = "Unhide reply", "unhide"
		}
		c.body = "Hiding only works on a reply to one of your posts."
	case "delete":
		if tw.Author.ID != m.me.ID {
			m.status = "you can only delete your own posts"
			m.statusErr = true
			return m, nil, true
		}
		c.want = true
		c.title, c.yes = "Delete post", "delete"
		c.body = preview(tw)
	default:
		return m, nil, true
	}
	m.confirm = c
	m.mode = modeConfirm
	m.status = ""
	return m, nil, true
}

func (m Model) runConfirm() (tea.Model, tea.Cmd) {
	c := m.confirm
	m.mode = modeMain
	tw := c.tweet
	switch c.action {
	case "repost":
		m = m.applyRepost(tw.ID, tw.ActionID(), c.want)
		if c.want {
			m.status = "reposted"
		} else {
			m.status = "repost removed"
		}
		next, cmd, _ := m.act("repost", tw, c.want, func(ctx context.Context, id string) error {
			if c.want {
				return m.svc.Repost(ctx, id)
			}
			return m.svc.Unrepost(ctx, id)
		})
		return next, cmd
	case "follow":
		m = m.setAuthor(tw.Author.ID, func(u *xapi.User) { u.Following = c.want })
		m.status = c.yes + " " + tw.Author.Handle()
		return m.userAct("follow", tw, c.want, func(ctx context.Context, id string) error {
			if c.want {
				return m.svc.Follow(ctx, id)
			}
			return m.svc.Unfollow(ctx, id)
		})
	case "mute":
		m = m.setAuthor(tw.Author.ID, func(u *xapi.User) { u.Muting = c.want })
		m.status = c.yes + " " + tw.Author.Handle()
		return m.userAct("mute", tw, c.want, func(ctx context.Context, id string) error {
			if c.want {
				return m.svc.Mute(ctx, id)
			}
			return m.svc.Unmute(ctx, id)
		})
	case "block":
		m = m.setAuthor(tw.Author.ID, func(u *xapi.User) { u.Blocking = c.want })
		m.status = c.yes + " " + tw.Author.Handle()
		return m.userAct("block", tw, c.want, func(ctx context.Context, id string) error {
			if c.want {
				return m.svc.Block(ctx, id)
			}
			return m.svc.Unblock(ctx, id)
		})
	case "hide":
		m = m.walk(tw.ID, tw.ActionID(), func(t *xapi.Tweet) { t.Hidden = c.want })
		if c.want {
			m.status = "reply hidden"
		} else {
			m.status = "reply visible"
		}
		next, cmd, _ := m.act("hide", tw, c.want, func(ctx context.Context, id string) error {
			return m.svc.HideReply(ctx, id, c.want)
		})
		return next, cmd
	case "delete":
		m.status = "deleting…"
		var seq int
		m, seq = m.bump("delete", tw.ID)
		svc := m.svc
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			err := svc.Delete(ctx, tw.ID)
			return actionResult{op: "delete", rowID: tw.ID, actionID: tw.ID, seq: seq, want: true, err: err, deleted: err == nil}
		}
	default:
		return m, nil
	}
}

func (m Model) userAct(op string, tw xapi.Tweet, want bool, fn func(context.Context, string) error) (tea.Model, tea.Cmd) {
	var seq int
	m, seq = m.bump(op, tw.Author.ID)
	uid := tw.Author.ID
	row := tw.ID
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		err := fn(ctx, uid)
		return actionResult{op: op, rowID: row, actionID: uid, seq: seq, want: want, err: err}
	}
}

func (m Model) copyLink() (Model, tea.Cmd, bool) {
	m, tw, ok := m.needTweet()
	if !ok {
		return m, nil, true
	}
	link := tw.URL()
	return m, func() tea.Msg {
		if err := copyText(link); err != nil {
			return notice{text: err.Error(), isErr: true}
		}
		return notice{text: "copied " + link}
	}, true
}

func (m Model) openBrowser() (Model, tea.Cmd, bool) {
	m, tw, ok := m.needTweet()
	if !ok {
		return m, nil, true
	}
	link := tw.URL()
	return m, func() tea.Msg {
		if err := openURL(link); err != nil {
			return notice{text: err.Error(), isErr: true}
		}
		return notice{text: "opened " + link}
	}, true
}

func (m Model) openCompose(kind string) (Model, tea.Cmd, bool) {
	var tw xapi.Tweet
	if kind != "post" {
		var ok bool
		m, tw, ok = m.needTweet()
		if !ok {
			return m, nil, true
		}
	}
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Placeholder = "What's happening?"
	ta.CharLimit = 25000
	w := m.dialogWidth() - 6
	if w < 16 {
		w = 16
	}
	ta.SetWidth(w)
	ta.SetHeight(5)
	cmd := ta.Focus()
	m.ta = ta
	m.composeKind = kind
	m.composeReply, m.composeQuote, m.composeWho = "", "", ""
	switch kind {
	case "reply":
		m.composeReply = tw.ActionID()
		m.composeWho = tw.Author.Handle()
	case "quote":
		m.composeQuote = tw.ActionID()
		m.composeWho = tw.Author.Handle()
	default:
		m.composeKind = "post"
	}
	m.mode = modeCompose
	m.status = ""
	return m, cmd, true
}

func (m Model) submitCompose() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.ta.Value())
	if text == "" {
		m.status = "write something first"
		m.statusErr = true
		return m, nil
	}
	kind := m.composeKind
	reply := m.composeReply
	quote := m.composeQuote
	svc := m.svc
	m.mode = modeMain
	m.ta.Blur()
	m.status = "posting…"
	m.statusErr = false
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		tw, err := svc.Post(ctx, text, reply, quote)
		if err == nil && tw.Text == "" {
			tw.Text = text
		}
		return actionResult{op: "post", err: err, tweet: tw, hasTweet: err == nil, want: kind == "reply"}
	}
}

func (m Model) startSearch() (Model, tea.Cmd, bool) {
	m.search.SetValue("")
	m.search.Width = m.dialogWidth() - 6
	if m.search.Width > 56 {
		m.search.Width = 56
	}
	cmd := m.search.Focus()
	m.mode = modeSearch
	m.status = ""
	return m, cmd, true
}

func (m Model) submitSearch() (tea.Model, tea.Cmd) {
	q := strings.TrimSpace(m.search.Value())
	m.search.Blur()
	m.mode = modeMain
	if q == "" {
		return m, nil
	}
	m = m.push()
	m.kind = feedSearch
	m.arg = q
	m.label = "search · " + q
	m.side = -1
	m.tweets = nil
	m.cursor, m.offset = 0, 0
	m.thread, m.threadID = nil, ""
	return m.reload(false)
}

func (m Model) editClientID() (tea.Model, tea.Cmd) {
	if m.store == nil {
		m.status = "config store is unavailable"
		m.statusErr = true
		return m, nil
	}
	if strings.TrimSpace(m.clientID.Value()) == "" {
		m.clientID.SetValue(m.store.Get().ClientID)
	}
	cmd := m.clientID.Focus()
	m.mode = modeEditID
	return m, cmd
}

func (m Model) editSecret() (tea.Model, tea.Cmd) {
	if m.store == nil {
		m.status = "config store is unavailable"
		m.statusErr = true
		return m, nil
	}
	m.secret.SetValue("")
	cmd := m.secret.Focus()
	m.mode = modeEditSecret
	return m, cmd
}

func (m Model) saveClientID() (tea.Model, tea.Cmd) {
	v := strings.TrimSpace(m.clientID.Value())
	m.clientID.Blur()
	if v == "" {
		m.mode = modeEditID
		m.status = "paste the client id"
		m.statusErr = true
		return m, m.clientID.Focus()
	}
	if err := m.store.Update(func(c *config.Config) { c.ClientID = v }); err != nil {
		m.mode = modeEditID
		m.status = err.Error()
		m.statusErr = true
		return m, m.clientID.Focus()
	}
	m.status = ""
	m.statusErr = false
	return m.beginAuth()
}

func (m Model) saveSecret() (tea.Model, tea.Cmd) {
	v := strings.TrimSpace(m.secret.Value())
	m.secret.Blur()
	if err := m.store.Update(func(c *config.Config) { c.ClientSecret = v }); err != nil {
		m.mode = modeSetup
		m.status = err.Error()
		m.statusErr = true
		return m, nil
	}
	next, cmd := m.editClientID()
	edited := next.(Model)
	if v == "" {
		edited.status = "cleared the client secret"
	} else {
		edited.status = "saved the client secret"
	}
	edited.statusErr = false
	return edited, cmd
}

func (m Model) beginAuth() (Model, tea.Cmd) {
	if m.store == nil {
		m.status = "config store is unavailable"
		m.statusErr = true
		return m, nil
	}
	cfg := m.store.Get()
	if strings.TrimSpace(cfg.ClientID) == "" {
		next, cmd := m.editClientID()
		edited := next.(Model)
		edited.status = "paste the client id, then press enter"
		edited.statusErr = false
		return edited, cmd
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	m.authCancel = cancel
	m.mode = modeAuthWait
	m.authURL = ""
	m.authTicks = 0
	m.status = "waiting for the browser"
	m.statusErr = false
	store := m.store
	urlCh := make(chan string, 1)
	return m, tea.Batch(
		func() tea.Msg {
			select {
			case u := <-urlCh:
				return authURLMsg{url: u}
			case <-ctx.Done():
				return authURLMsg{}
			}
		},
		func() tea.Msg {
			defer cancel()
			tok, err := xapi.Authorize(ctx, cfg.ClientID, cfg.ClientSecret, cfg.Port(), func(u string) {
				select {
				case urlCh <- u:
				default:
				}
			})
			if err != nil {
				return authDone{err: err}
			}
			if err := store.Update(func(c *config.Config) {
				c.AccessToken = tok.AccessToken
				c.RefreshToken = tok.RefreshToken
				c.Expiry = tok.Expiry
			}); err != nil {
				return authDone{err: err}
			}
			client := xapi.New(xapi.Options{
				AccessToken:  tok.AccessToken,
				RefreshToken: tok.RefreshToken,
				Expiry:       tok.Expiry,
				ClientID:     cfg.ClientID,
				ClientSecret: cfg.ClientSecret,
				Persist: func(t xapi.Token) {
					_ = store.Update(func(c *config.Config) {
						c.AccessToken = t.AccessToken
						if t.RefreshToken != "" {
							c.RefreshToken = t.RefreshToken
						}
						c.Expiry = t.Expiry
					})
				},
			})
			meCtx, meCancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer meCancel()
			me, err := client.Me(meCtx)
			if err != nil {
				return authDone{err: err}
			}
			return authDone{svc: client, me: me}
		},
		authTick(),
	)
}

func authTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return authTickMsg{} })
}

func (m Model) onMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	switch m.mode {
	case modeSetup:
		m.status = "Press enter to sign in. i changes the client id. q quits"
		m.statusErr = false
		return m, nil
	case modeAuthWait:
		m.status = "Waiting for the browser. esc cancels. o opens the page again"
		m.statusErr = false
		return m, nil
	default:
		return m, nil
	}
}

func (m Model) enterDemo() (tea.Model, tea.Cmd) {
	d := xapi.NewDemo()
	me, err := d.Me(context.Background())
	if err != nil {
		m.status = err.Error()
		m.statusErr = true
		return m, nil
	}
	m.svc = d
	m.me = me
	m.demo = true
	m.mode = modeMain
	m.kind = feedHome
	m.arg = ""
	m.label = "home"
	m.side = 0
	m.stack = nil
	m.tweets = nil
	m.gen = 0
	m.status = "demo feed"
	m.statusErr = false
	return m.reload(false)
}

func (m Model) onAuth(msg authDone) (tea.Model, tea.Cmd) {
	m.authCancel = nil
	m.authURL = ""
	m.authTicks = 0
	if msg.err != nil {
		m.mode = modeSetup
		m.statusErr = true
		switch {
		case errors.Is(msg.err, context.Canceled):
			m.status = "sign-in cancelled"
			m.statusErr = false
		case errors.Is(msg.err, context.DeadlineExceeded):
			m.status = "sign-in timed out. Press a to try again"
		default:
			m.status = msg.err.Error()
		}
		return m, nil
	}
	m.svc = msg.svc
	m.me = msg.me
	m.demo = false
	m.mode = modeMain
	m.kind = feedHome
	m.arg = ""
	m.label = "home"
	m.side = 0
	m.gen = 0
	m.status = "signed in as " + msg.me.Handle()
	m.statusErr = false
	return m.reload(false)
}

func (m Model) onFeed(msg feedLoaded) Model {
	if msg.gen != m.gen {
		return m
	}
	m.loading = false
	m = m.noteRate(msg.rate)
	if msg.err != nil {
		m.status = msg.err.Error()
		m.statusErr = true
		return m
	}
	if strings.HasPrefix(m.status, "loading") {
		m.status = ""
		m.statusErr = false
	}
	tweets := m.annotate(msg.tweets)
	if msg.append {
		seen := map[string]bool{}
		for _, t := range m.tweets {
			seen[t.ID] = true
		}
		for _, t := range tweets {
			if !seen[t.ID] {
				m.tweets = append(m.tweets, t)
			}
		}
	} else {
		prev := ""
		if m.cursor >= 0 && m.cursor < len(m.tweets) {
			prev = m.tweets[m.cursor].ID
		}
		m.tweets = tweets
		m.cursor, m.offset = 0, 0
		if prev != "" {
			for i, t := range m.tweets {
				if t.ID == prev {
					m.cursor = i
					break
				}
			}
		}
	}
	m.next = msg.next
	return m.ensureCursor()
}

func (m Model) onLists(msg listsLoaded) Model {
	if msg.gen != m.gen {
		return m
	}
	m.loading = false
	m = m.noteRate(msg.rate)
	if msg.err != nil {
		m.status = msg.err.Error()
		m.statusErr = true
		return m
	}
	if strings.HasPrefix(m.status, "loading") {
		m.status = ""
	}
	m.lists = msg.lists
	m.tweets = nil
	m.next = ""
	m.cursor, m.offset = 0, 0
	return m.ensureCursor()
}

func (m Model) onThread(msg threadLoaded) Model {
	if msg.id != m.threadID {
		return m
	}
	if msg.err != nil {
		m.status = msg.err.Error()
		m.statusErr = true
	} else if strings.HasPrefix(m.status, "loading thread") {
		m.status = ""
	}
	if len(msg.tweets) > 0 {
		m.thread = msg.tweets
	}
	m.detailOff = 0
	return m
}

func (m Model) onAction(msg actionResult) Model {
	key := msg.op + ":" + msg.rowID
	if msg.op == "follow" || msg.op == "mute" || msg.op == "block" {
		key = msg.op + ":" + msg.actionID
	}
	if msg.op != "post" && m.seq[key] != msg.seq {
		return m
	}
	m = m.noteRate("")
	if msg.err != nil {
		m.status = msg.err.Error()
		m.statusErr = true
		switch msg.op {
		case "like":
			m = m.applyLike(msg.rowID, msg.actionID, !msg.want)
		case "bookmark":
			m = m.walk(msg.rowID, msg.actionID, func(t *xapi.Tweet) { t.Bookmarked = !msg.want })
		case "repost":
			m = m.applyRepost(msg.rowID, msg.actionID, !msg.want)
		case "hide":
			m = m.walk(msg.rowID, msg.actionID, func(t *xapi.Tweet) { t.Hidden = !msg.want })
		case "follow":
			m = m.setAuthor(msg.actionID, func(u *xapi.User) { u.Following = !msg.want })
		case "mute":
			m = m.setAuthor(msg.actionID, func(u *xapi.User) { u.Muting = !msg.want })
		case "block":
			m = m.setAuthor(msg.actionID, func(u *xapi.User) { u.Blocking = !msg.want })
		}
		return m
	}
	if msg.deleted {
		m.tweets = removeTweet(m.tweets, msg.rowID)
		m.thread = removeTweet(m.thread, msg.rowID)
		m.status = "deleted"
		m.statusErr = false
		return m.ensureCursor()
	}
	if msg.hasTweet {
		tw := msg.tweet
		if tw.Author.ID == "" {
			tw.Author = m.me
		}
		if tw.CreatedAt.IsZero() {
			tw.CreatedAt = m.clock()
		}
		if m.kind == feedHome || (m.kind == feedUser && (m.arg == "" || m.arg == m.me.ID)) {
			m.tweets = append([]xapi.Tweet{tw}, m.tweets...)
			m.cursor, m.offset = 0, 0
		}
		if m.threadID != "" {
			m.thread = append(m.thread, tw)
		}
		m.status = "posted"
		m.statusErr = false
	}
	return m
}

func removeTweet(in []xapi.Tweet, id string) []xapi.Tweet {
	out := in[:0:0]
	for _, t := range in {
		if t.ID != id {
			out = append(out, t)
		}
	}
	if out == nil {
		return []xapi.Tweet{}
	}
	return out
}

func (m Model) applyLike(rowID, actionID string, want bool) Model {
	return m.walk(rowID, actionID, func(t *xapi.Tweet) {
		if t.Liked == want {
			return
		}
		t.Liked = want
		if want {
			t.Metrics.Likes++
		} else if t.Metrics.Likes > 0 {
			t.Metrics.Likes--
		}
	})
}

func (m Model) applyRepost(rowID, actionID string, want bool) Model {
	return m.walk(rowID, actionID, func(t *xapi.Tweet) {
		if t.Reposted == want {
			return
		}
		t.Reposted = want
		if want {
			t.Metrics.Reposts++
		} else if t.Metrics.Reposts > 0 {
			t.Metrics.Reposts--
		}
	})
}

func (m Model) walk(rowID, actionID string, fn func(*xapi.Tweet)) Model {
	touch := func(list []xapi.Tweet) {
		for i := range list {
			if list[i].ID == rowID || (actionID != "" && list[i].ActionID() == actionID) {
				fn(&list[i])
			}
		}
	}
	touch(m.tweets)
	touch(m.thread)
	return m
}

func (m Model) setAuthor(userID string, fn func(*xapi.User)) Model {
	if userID == m.me.ID {
		fn(&m.me)
	}
	touch := func(list []xapi.Tweet) {
		for i := range list {
			if list[i].Author.ID == userID {
				fn(&list[i].Author)
			}
			if list[i].RepostedBy.ID == userID {
				fn(&list[i].RepostedBy)
			}
		}
	}
	touch(m.tweets)
	touch(m.thread)
	return m
}

func preview(t xapi.Tweet) string {
	text := strings.Join(strings.Fields(t.Text), " ")
	return t.Author.Handle() + "  " + cut(text, 72)
}

func (m Model) detailPage() int {
	h := m.timelineBodyHeight()
	if h < 1 {
		return 1
	}
	return h
}

func fmtCount(n int) string {
	return fmt.Sprintf("%d", n)
}
