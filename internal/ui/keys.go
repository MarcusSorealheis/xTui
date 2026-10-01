package ui

// Binding is one keyboard action. Keys are bubbletea KeyMsg strings.
type binding struct {
	Action string
	Keys   []string
	Help   string
	Group  string
}

const (
	actNext     = "next"
	actPrev     = "prev"
	actTop      = "top"
	actBottom   = "bottom"
	actPageDown = "page-down"
	actPageUp   = "page-up"
	actPaneNext = "pane-next"
	actPanePrev = "pane-prev"
	actHome     = "home"
	actMentions = "mentions"
	actMarks    = "bookmarks"
	actLikes    = "likes"
	actPosts    = "posts"
	actLists    = "lists"
	actOpen     = "open"
	actBack     = "back"
	actRefresh  = "refresh"
	actLike     = "like"
	actRepost   = "repost"
	actQuote    = "quote"
	actReply    = "reply"
	actBookmark = "bookmark"
	actFollow   = "follow"
	actMute     = "mute"
	actBlock    = "block"
	actHide     = "hide"
	actDelete   = "delete"
	actCopy     = "copy"
	actBrowser  = "browser"
	actAuthor   = "author"
	actCompose  = "compose"
	actSearch   = "search"
	actHelp     = "help"
	actQuit     = "quit"
	actSignOut  = "sign-out"
)

var bindings = []binding{
	{actNext, []string{"j", "down"}, "next", "Navigate"},
	{actPrev, []string{"k", "up"}, "previous", "Navigate"},
	{actTop, []string{"g"}, "first", "Navigate"},
	{actBottom, []string{"G"}, "last", "Navigate"},
	{actPageDown, []string{"ctrl+d", "pgdown"}, "page down", "Navigate"},
	{actPageUp, []string{"ctrl+u", "pgup"}, "page up", "Navigate"},
	{actPaneNext, []string{"tab"}, "next pane", "Navigate"},
	{actPanePrev, []string{"shift+tab"}, "previous pane", "Navigate"},
	{actHome, []string{"1"}, "home", "Feeds"},
	{actMentions, []string{"2"}, "mentions", "Feeds"},
	{actMarks, []string{"3"}, "bookmarks", "Feeds"},
	{actLikes, []string{"4"}, "likes", "Feeds"},
	{actPosts, []string{"5"}, "your posts", "Feeds"},
	{actLists, []string{"6"}, "lists", "Feeds"},
	{actOpen, []string{"enter"}, "open", "Navigate"},
	{actBack, []string{"esc"}, "back", "Navigate"},
	{actRefresh, []string{"R"}, "refresh", "Navigate"},
	{actLike, []string{"l"}, "like", "Post"},
	{actRepost, []string{"t"}, "repost", "Post"},
	{actQuote, []string{"T"}, "quote", "Post"},
	{actReply, []string{"r"}, "reply", "Post"},
	{actBookmark, []string{"b"}, "bookmark", "Post"},
	{actFollow, []string{"f"}, "follow", "Author"},
	{actMute, []string{"m"}, "mute", "Author"},
	{actBlock, []string{"B"}, "block", "Author"},
	{actHide, []string{"h"}, "hide reply", "Post"},
	{actDelete, []string{"d"}, "delete", "Post"},
	{actCopy, []string{"c"}, "copy link", "Post"},
	{actBrowser, []string{"o"}, "open in browser", "Post"},
	{actAuthor, []string{"u"}, "author posts", "Author"},
	{actCompose, []string{"n"}, "new post", "Post"},
	{actSearch, []string{"/"}, "search", "Feeds"},
	{actHelp, []string{"?"}, "help", "App"},
	{actQuit, []string{"q", "ctrl+c"}, "quit", "App"},
	{actSignOut, []string{"ctrl+x"}, "sign out", "App"},
}

func actionFor(key string) (string, bool) {
	for _, b := range bindings {
		for _, k := range b.Keys {
			if k == key {
				return b.Action, true
			}
		}
	}
	return "", false
}

type footItem struct {
	keys  string
	label string
}

func mainFooter() []footItem {
	return []footItem{
		{"j/k", "move"},
		{"enter", "open"},
		{"l", "like"},
		{"t", "repost"},
		{"T", "quote"},
		{"r", "reply"},
		{"b", "bookmark"},
		{"n", "post"},
		{"?", "help"},
		{"q", "quit"},
		{"/", "search"},
		{"f", "follow"},
		{"m", "mute"},
		{"o", "open url"},
		{"R", "refresh"},
	}
}
