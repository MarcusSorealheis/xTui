package xapi

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Demo is an in-memory feed used by --demo and the UI tests.
type Demo struct {
	mu         sync.Mutex
	me         User
	home       []Tweet
	mentions   []Tweet
	lists      []List
	listTweets map[string][]Tweet
	pageSize   int
}

// NewDemo returns a signed-in demo user with a small home timeline.
func NewDemo() *Demo {
	now := time.Now()
	me := User{ID: "1", Name: "Ada Lovelace", Username: "ada", Description: "notes on engines", Verified: true}
	grace := User{ID: "2", Name: "Grace Hopper", Username: "grace", Verified: true, Description: "compilers"}
	linus := User{ID: "3", Name: "Linus", Username: "linus"}
	octo := User{ID: "4", Name: "Octo", Username: "octo", Following: true}
	d := &Demo{
		me:       me,
		pageSize: 4,
		lists: []List{
			{ID: "l1", Name: "build systems", Description: "remote execution and caches", Members: 42, Owner: "ada"},
			{ID: "l2", Name: "terminals", Description: "tuis worth stealing from", Members: 18, Owner: "octo"},
		},
		listTweets: map[string][]Tweet{},
	}
	d.home = []Tweet{
		{
			ID: "101", Text: "cache coherency is a social problem pretending to be a protocol",
			CreatedAt: now.Add(-2 * time.Minute), Author: grace, ConversationID: "101",
			Metrics: Metrics{Likes: 128, Reposts: 17, Replies: 9, Quotes: 2, Bookmarks: 14},
		},
		{
			ID: "102", Text: "the diff is the documentation. if the diff is lonely, write the sentence it replaced.",
			CreatedAt: now.Add(-48 * time.Minute), Author: linus, ConversationID: "102",
			Metrics: Metrics{Likes: 64, Reposts: 4, Replies: 1}, Liked: true,
		},
		{
			ID: "103", Text: "shipped the scheduler behind a flag. the flag is the feature.",
			CreatedAt: now.Add(-3 * time.Hour), Author: grace, ConversationID: "90",
			Metrics:  Metrics{Likes: 40, Reposts: 6, Replies: 3},
			IsRepost: true, RepostedBy: octo,
			Refs: []Ref{{Type: "retweeted", ID: "103", Text: "shipped the scheduler behind a flag. the flag is the feature.", Author: "grace"}},
		},
		{
			ID: "104", Text: "replying from the train. the timeline is a worse newspaper and a better notebook.",
			CreatedAt: now.Add(-5 * time.Hour), Author: octo, ConversationID: "101",
			Metrics: Metrics{Likes: 12, Reposts: 1, Replies: 0},
			Refs:    []Ref{{Type: "replied_to", ID: "101", Author: "grace", Text: "cache coherency is a social problem pretending to be a protocol"}},
		},
		{
			ID: "105", Text: "quoting this because the second sentence is the one I needed.",
			CreatedAt: now.Add(-26 * time.Hour), Author: me, ConversationID: "105",
			Metrics: Metrics{Likes: 8, Reposts: 0, Replies: 1, Quotes: 0},
			Refs:    []Ref{{Type: "quoted", ID: "102", Author: "linus", Text: "the diff is the documentation. if the diff is lonely, write the sentence it replaced."}},
		},
		{
			ID: "106", Text: "a photo of the whiteboard after the incident review",
			CreatedAt: now.Add(-3 * 24 * time.Hour), Author: grace, ConversationID: "106",
			Metrics:   Metrics{Likes: 210, Reposts: 22, Replies: 30},
			Media:     []Media{{Type: "photo", URL: "https://example.invalid/board.jpg", Alt: "a whiteboard full of arrows"}},
			Sensitive: false,
		},
		{
			ID: "107", Text: "long note: a terminal client should put every verb on a key. like, repost, quote, reply, bookmark, follow, mute, block, hide, delete, copy, open. the mouse is a rumor.",
			CreatedAt: now.Add(-6 * 24 * time.Hour), Author: me, ConversationID: "107",
			Metrics: Metrics{Likes: 3, Reposts: 1, Replies: 0, Bookmarks: 2}, Bookmarked: true,
		},
	}
	d.mentions = []Tweet{
		{
			ID: "201", Text: "@ada the home timeline endpoint is reverse chronological. the ranked one is a different product.",
			CreatedAt: now.Add(-20 * time.Minute), Author: linus, ConversationID: "201",
			Metrics: Metrics{Likes: 5, Replies: 1},
		},
	}
	d.listTweets["l1"] = []Tweet{d.home[0], d.home[2]}
	d.listTweets["l2"] = []Tweet{d.home[6]}
	return d
}

// Me returns the demo user.
func (d *Demo) Me(context.Context) (User, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.me, nil
}

// Rate labels the demo so the header can show it.
func (d *Demo) Rate() string { return "demo" }

// Timeline serves the demo pages. The home feed pages at pageSize.
func (d *Demo) Timeline(_ context.Context, feed Feed, arg, page string) (Timeline, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch feed {
	case FeedHome:
		return pageTweets(d.home, d.pageSize, page), nil
	case FeedMentions:
		return Timeline{Tweets: cloneTweets(d.mentions)}, nil
	case FeedBookmarks:
		return Timeline{Tweets: filter(d.home, func(t Tweet) bool { return t.Bookmarked })}, nil
	case FeedLikes:
		return Timeline{Tweets: filter(d.home, func(t Tweet) bool { return t.Liked })}, nil
	case FeedUser:
		if arg == "" {
			arg = d.me.ID
		}
		return Timeline{Tweets: filter(append(cloneTweets(d.home), d.mentions...), func(t Tweet) bool {
			return t.Author.ID == arg
		})}, nil
	case FeedList:
		return Timeline{Tweets: cloneTweets(d.listTweets[arg])}, nil
	case FeedSearch:
		q := strings.ToLower(strings.TrimPrefix(arg, "conversation_id:"))
		if strings.HasPrefix(arg, "conversation_id:") {
			return Timeline{Tweets: filter(d.home, func(t Tweet) bool {
				return t.ConversationID == q || t.ID == q
			})}, nil
		}
		return Timeline{Tweets: filter(d.home, func(t Tweet) bool {
			return strings.Contains(strings.ToLower(t.Text), strings.ToLower(arg)) ||
				strings.Contains(strings.ToLower(t.Author.Username), strings.ToLower(arg))
		})}, nil
	default:
		return Timeline{}, fmt.Errorf("unknown feed")
	}
}

// Lists returns the demo lists.
func (d *Demo) Lists(context.Context) ([]List, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]List, len(d.lists))
	copy(out, d.lists)
	return out, nil
}

// Thread returns the conversation in time order.
func (d *Demo) Thread(_ context.Context, tweet Tweet) ([]Tweet, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	conv := tweet.ConversationID
	if conv == "" {
		conv = tweet.ID
	}
	out := filter(append(cloneTweets(d.home), d.mentions...), func(t Tweet) bool {
		return t.ConversationID == conv || t.ID == conv
	})
	if len(out) == 0 {
		out = []Tweet{tweet}
	}
	sortByTime(out)
	return out, nil
}

// Like marks a post liked.
func (d *Demo) Like(_ context.Context, id string) error {
	return d.setFlag(id, func(t *Tweet) {
		if !t.Liked {
			t.Metrics.Likes++
		}
		t.Liked = true
	})
}

// Unlike clears a like.
func (d *Demo) Unlike(_ context.Context, id string) error {
	return d.setFlag(id, func(t *Tweet) {
		if t.Liked && t.Metrics.Likes > 0 {
			t.Metrics.Likes--
		}
		t.Liked = false
	})
}

// Repost marks a post reposted.
func (d *Demo) Repost(_ context.Context, id string) error {
	return d.setFlag(id, func(t *Tweet) {
		if !t.Reposted {
			t.Metrics.Reposts++
		}
		t.Reposted = true
	})
}

// Unrepost clears a repost.
func (d *Demo) Unrepost(_ context.Context, id string) error {
	return d.setFlag(id, func(t *Tweet) {
		if t.Reposted && t.Metrics.Reposts > 0 {
			t.Metrics.Reposts--
		}
		t.Reposted = false
	})
}

// Bookmark marks a post bookmarked.
func (d *Demo) Bookmark(_ context.Context, id string) error {
	return d.setFlag(id, func(t *Tweet) { t.Bookmarked = true })
}

// Unbookmark clears a bookmark.
func (d *Demo) Unbookmark(_ context.Context, id string) error {
	return d.setFlag(id, func(t *Tweet) { t.Bookmarked = false })
}

// Follow marks the author followed.
func (d *Demo) Follow(_ context.Context, userID string) error {
	return d.setUser(userID, func(u *User) { u.Following = true })
}

// Unfollow clears follow.
func (d *Demo) Unfollow(_ context.Context, userID string) error {
	return d.setUser(userID, func(u *User) { u.Following = false })
}

// Mute marks the author muted.
func (d *Demo) Mute(_ context.Context, userID string) error {
	return d.setUser(userID, func(u *User) { u.Muting = true })
}

// Unmute clears mute.
func (d *Demo) Unmute(_ context.Context, userID string) error {
	return d.setUser(userID, func(u *User) { u.Muting = false })
}

// Block marks the author blocked.
func (d *Demo) Block(_ context.Context, userID string) error {
	return d.setUser(userID, func(u *User) { u.Blocking = true })
}

// Unblock clears block.
func (d *Demo) Unblock(_ context.Context, userID string) error {
	return d.setUser(userID, func(u *User) { u.Blocking = false })
}

// HideReply marks a reply hidden.
func (d *Demo) HideReply(_ context.Context, id string, hidden bool) error {
	return d.setFlag(id, func(t *Tweet) { t.Hidden = hidden })
}

// Post appends a post to the home timeline.
func (d *Demo) Post(_ context.Context, text, replyTo, quoteID string) (Tweet, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if strings.TrimSpace(text) == "" {
		return Tweet{}, fmt.Errorf("text is empty")
	}
	tw := Tweet{
		ID:             fmt.Sprintf("p%d", time.Now().UnixNano()),
		Text:           text,
		CreatedAt:      time.Now(),
		Author:         d.me,
		ConversationID: replyTo,
		Metrics:        Metrics{},
	}
	if tw.ConversationID == "" {
		tw.ConversationID = tw.ID
	}
	if replyTo != "" {
		tw.Refs = append(tw.Refs, Ref{Type: "replied_to", ID: replyTo})
	}
	if quoteID != "" {
		tw.Refs = append(tw.Refs, Ref{Type: "quoted", ID: quoteID})
	}
	d.home = append([]Tweet{tw}, d.home...)
	return tw, nil
}

// Delete removes a post the demo user wrote.
func (d *Demo) Delete(_ context.Context, id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	var next []Tweet
	found := false
	for _, t := range d.home {
		if t.ID == id {
			found = true
			if t.Author.ID != d.me.ID {
				return fmt.Errorf("you can only delete your own posts")
			}
			continue
		}
		next = append(next, t)
	}
	if !found {
		return fmt.Errorf("post not found")
	}
	d.home = next
	return nil
}

func (d *Demo) setFlag(id string, fn func(*Tweet)) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	hit := false
	for i := range d.home {
		if d.home[i].ID == id || d.home[i].ActionID() == id {
			fn(&d.home[i])
			hit = true
		}
	}
	for i := range d.mentions {
		if d.mentions[i].ID == id || d.mentions[i].ActionID() == id {
			fn(&d.mentions[i])
			hit = true
		}
	}
	if !hit {
		return fmt.Errorf("post not found")
	}
	return nil
}

func (d *Demo) setUser(id string, fn func(*User)) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	hit := false
	touch := func(u *User) {
		if u.ID == id {
			fn(u)
			hit = true
		}
	}
	touch(&d.me)
	for i := range d.home {
		touch(&d.home[i].Author)
		touch(&d.home[i].RepostedBy)
	}
	for i := range d.mentions {
		touch(&d.mentions[i].Author)
	}
	if !hit {
		return fmt.Errorf("account not found")
	}
	return nil
}

func pageTweets(all []Tweet, size int, page string) Timeline {
	if size <= 0 || size >= len(all) {
		return Timeline{Tweets: cloneTweets(all)}
	}
	if page == "" {
		return Timeline{Tweets: cloneTweets(all[:size]), NextToken: "p2"}
	}
	if size >= len(all) {
		return Timeline{}
	}
	return Timeline{Tweets: cloneTweets(all[size:])}
}

func filter(in []Tweet, keep func(Tweet) bool) []Tweet {
	var out []Tweet
	for _, t := range in {
		if keep(t) {
			out = append(out, t)
		}
	}
	return out
}

func cloneTweets(in []Tweet) []Tweet {
	out := make([]Tweet, len(in))
	copy(out, in)
	return out
}
