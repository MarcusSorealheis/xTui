package ui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/MarcusSorealheis/xTui/internal/config"
	"github.com/MarcusSorealheis/xTui/internal/xapi"
)

type mode int

const (
	modeMain mode = iota
	modeSetup
	modeEditID
	modeEditSecret
	modeAuthWait
	modeCompose
	modeConfirm
	modeHelp
	modeSearch
)

type focus int

const (
	focusTimeline focus = iota
	focusSide
	focusDetail
)

type feedKind int

const (
	feedHome feedKind = iota
	feedMentions
	feedBookmarks
	feedLikes
	feedUser
	feedLists
	feedListTweets
	feedSearch
)

type sideFeed struct {
	key   string
	label string
	kind  feedKind
}

var sideFeeds = []sideFeed{
	{"1", "home", feedHome},
	{"2", "mentions", feedMentions},
	{"3", "bookmarks", feedBookmarks},
	{"4", "likes", feedLikes},
	{"5", "posts", feedUser},
	{"6", "lists", feedLists},
}

type frame struct {
	kind     feedKind
	arg      string
	label    string
	tweets   []xapi.Tweet
	lists    []xapi.List
	cursor   int
	offset   int
	next     string
	side     int
	thread   []xapi.Tweet
	threadID string
}

type confirmState struct {
	title  string
	body   string
	yes    string
	action string
	tweet  xapi.Tweet
	want   bool
}

// Options configures the root model.
type Options struct {
	Service xapi.Service
	Me      xapi.User
	Store   *config.Store
	Demo    bool
	Err     string
}

// Model is the xTui screen.
type Model struct {
	svc   xapi.Service
	store *config.Store
	me    xapi.User
	demo  bool

	width    int
	height   int
	focus    focus
	mode     mode
	helpFrom mode

	kind    feedKind
	arg     string
	label   string
	tweets  []xapi.Tweet
	lists   []xapi.List
	cursor  int
	offset  int
	next    string
	side    int
	stack   []frame
	gen     int
	loading bool
	rate    string

	thread    []xapi.Tweet
	threadID  string
	detailOff int

	status    string
	statusErr bool
	seq       map[string]int

	confirm      confirmState
	ta           textarea.Model
	composeKind  string
	composeReply string
	composeQuote string
	composeWho   string

	search   textinput.Model
	clientID textinput.Model
	secret   textinput.Model

	authCancel context.CancelFunc
	authURL    string
	authTicks  int
	now        func() time.Time
}

type feedLoaded struct {
	gen    int
	tweets []xapi.Tweet
	next   string
	append bool
	err    error
	rate   string
}

type listsLoaded struct {
	gen   int
	lists []xapi.List
	err   error
	rate  string
}

type threadLoaded struct {
	id     string
	tweets []xapi.Tweet
	err    error
}

type actionResult struct {
	op       string
	rowID    string
	actionID string
	seq      int
	want     bool
	err      error
	tweet    xapi.Tweet
	hasTweet bool
	deleted  bool
}

type notice struct {
	text  string
	isErr bool
}

type authDone struct {
	svc xapi.Service
	me  xapi.User
	err error
}

type authURLMsg struct {
	url string
}

type authTickMsg struct{}

type startSignInMsg struct{}

// New builds the screen. A nil service opens the sign-in screen.
func New(opts Options) Model {
	m := Model{
		svc:   opts.Service,
		store: opts.Store,
		me:    opts.Me,
		demo:  opts.Demo,
		focus: focusTimeline,
		kind:  feedHome,
		label: "home",
		seq:   map[string]int{},
		now:   time.Now,
	}
	m.search = textinput.New()
	m.search.Prompt = "/ "
	m.search.Placeholder = "search posts"
	m.search.CharLimit = 200
	m.clientID = textinput.New()
	m.clientID.Prompt = "client id "
	m.clientID.Placeholder = "paste the client id"
	m.clientID.CharLimit = 200
	m.clientID.Width = 48
	m.secret = textinput.New()
	m.secret.Prompt = "secret "
	m.secret.Placeholder = "optional, confidential clients only"
	m.secret.EchoMode = textinput.EchoPassword
	m.secret.EchoCharacter = '•'
	m.secret.CharLimit = 200
	m.secret.Width = 48
	if opts.Err != "" {
		m.status = opts.Err
		m.statusErr = true
	}
	if m.svc == nil {
		hasID := m.store != nil && strings.TrimSpace(m.store.Get().ClientID) != ""
		if !hasID && !m.statusErr {
			m.mode = modeEditID
			m.clientID.Focus()
			return m
		}
		m.mode = modeSetup
		return m
	}
	m.mode = modeMain
	m.loading = true
	m.gen = 1
	return m
}

// Init loads the home timeline, or continues sign-in when a client id is already saved.
func (m Model) Init() tea.Cmd {
	if m.svc != nil {
		return fetchCmd(m.svc, m.gen, m.kind, m.arg, "", false)
	}
	if m.store != nil && strings.TrimSpace(m.store.Get().ClientID) != "" && !m.statusErr {
		return func() tea.Msg { return startSignInMsg{} }
	}
	if m.mode == modeEditID {
		return m.clientID.Focus()
	}
	return nil
}

func fetchCmd(svc xapi.Service, gen int, kind feedKind, arg, page string, appendPage bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		if kind == feedLists {
			lists, err := svc.Lists(ctx)
			return listsLoaded{gen: gen, lists: lists, err: err, rate: svc.Rate()}
		}
		feed, feedArg, err := kindToFeed(kind, arg)
		if err != nil {
			return feedLoaded{gen: gen, err: err, append: appendPage}
		}
		tl, err := svc.Timeline(ctx, feed, feedArg, page)
		rate := ""
		if svc != nil {
			rate = svc.Rate()
		}
		return feedLoaded{
			gen: gen, tweets: tl.Tweets, next: tl.NextToken,
			append: appendPage, err: err, rate: rate,
		}
	}
}

func kindToFeed(kind feedKind, arg string) (xapi.Feed, string, error) {
	switch kind {
	case feedHome:
		return xapi.FeedHome, "", nil
	case feedMentions:
		return xapi.FeedMentions, "", nil
	case feedBookmarks:
		return xapi.FeedBookmarks, "", nil
	case feedLikes:
		return xapi.FeedLikes, "", nil
	case feedUser:
		return xapi.FeedUser, arg, nil
	case feedListTweets:
		return xapi.FeedList, arg, nil
	case feedSearch:
		return xapi.FeedSearch, arg, nil
	default:
		return 0, "", errString("nothing to load")
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func (m Model) reload(appendPage bool) (Model, tea.Cmd) {
	if m.svc == nil {
		m.status = "sign in first"
		m.statusErr = true
		return m, nil
	}
	if appendPage && (m.next == "" || m.loading) {
		return m, nil
	}
	m.gen++
	page := ""
	if appendPage {
		page = m.next
	} else {
		m.status = "loading " + m.label + "…"
		m.statusErr = false
	}
	m.loading = true
	return m, fetchCmd(m.svc, m.gen, m.kind, m.arg, page, appendPage)
}

func (m Model) clock() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

func (m Model) count() int {
	if m.kind == feedLists {
		return len(m.lists)
	}
	return len(m.tweets)
}

func (m Model) rowHeight() int {
	if m.kind == feedLists {
		return 2
	}
	return 3
}

func (m Model) visible() int {
	h := m.timelineBodyHeight()
	rh := m.rowHeight()
	if rh < 1 {
		rh = 1
	}
	n := h / rh
	if n < 1 {
		return 1
	}
	return n
}

func (m Model) selectedTweet() (xapi.Tweet, bool) {
	if m.kind == feedLists || m.cursor < 0 || m.cursor >= len(m.tweets) {
		return xapi.Tweet{}, false
	}
	return m.tweets[m.cursor], true
}

func (m Model) selectedList() (xapi.List, bool) {
	if m.kind != feedLists || m.cursor < 0 || m.cursor >= len(m.lists) {
		return xapi.List{}, false
	}
	return m.lists[m.cursor], true
}

func (m Model) ensureCursor() Model {
	n := m.count()
	if n == 0 {
		m.cursor = 0
		m.offset = 0
		return m
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	return m.reveal()
}

func (m Model) reveal() Model {
	vis := m.visible()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vis {
		m.offset = m.cursor - vis + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
	return m
}

func (m Model) annotate(in []xapi.Tweet) []xapi.Tweet {
	out := append([]xapi.Tweet(nil), in...)
	for i := range out {
		if m.kind == feedLikes {
			out[i].Liked = true
		}
		if m.kind == feedBookmarks {
			out[i].Bookmarked = true
		}
	}
	return out
}

func (m Model) push() Model {
	if len(m.stack) >= 8 {
		m.stack = m.stack[1:]
	}
	m.stack = append(m.stack, frame{
		kind: m.kind, arg: m.arg, label: m.label,
		tweets: append([]xapi.Tweet(nil), m.tweets...),
		lists:  append([]xapi.List(nil), m.lists...),
		cursor: m.cursor, offset: m.offset, next: m.next, side: m.side,
		thread: append([]xapi.Tweet(nil), m.thread...), threadID: m.threadID,
	})
	return m
}

func (m Model) pop() (Model, bool) {
	if len(m.stack) == 0 {
		return m, false
	}
	f := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	m.kind = f.kind
	m.arg = f.arg
	m.label = f.label
	m.tweets = f.tweets
	m.lists = f.lists
	m.cursor = f.cursor
	m.offset = f.offset
	m.next = f.next
	m.side = f.side
	m.thread = f.thread
	m.threadID = f.threadID
	m.loading = false
	return m, true
}

func quitCmd() tea.Cmd {
	return func() tea.Msg { return tea.Quit() }
}

func (m Model) bump(op, id string) (Model, int) {
	if m.seq == nil {
		m.seq = map[string]int{}
	}
	key := op + ":" + id
	m.seq[key]++
	return m, m.seq[key]
}

func (m Model) noteRate(rate string) Model {
	if rate != "" {
		m.rate = rate
	} else if m.svc != nil {
		if r := m.svc.Rate(); r != "" {
			m.rate = r
		}
	}
	return m
}
