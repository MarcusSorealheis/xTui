// Package xapi talks to the X API v2 on behalf of the signed-in user.
package xapi

import "time"

// User is an X account.
type User struct {
	ID          string
	Name        string
	Username    string
	Description string
	Verified    bool
	Following   bool
	FollowedBy  bool
	Muting      bool
	Blocking    bool
}

// DisplayName is the name, or the handle when the name is empty.
func (u User) DisplayName() string {
	if u.Name != "" {
		return u.Name
	}
	if u.Username != "" {
		return u.Username
	}
	return "unknown"
}

// Handle is @username.
func (u User) Handle() string {
	if u.Username == "" {
		return ""
	}
	return "@" + u.Username
}

// Metrics are the public counts on a post.
type Metrics struct {
	Likes     int
	Reposts   int
	Replies   int
	Quotes    int
	Bookmarks int
}

// Media is an attachment. URL may be empty for videos.
type Media struct {
	Type string
	URL  string
	Alt  string
}

// Ref is a post this post replies to, quotes, or reposts.
type Ref struct {
	Type   string
	ID     string
	Text   string
	Author string
}

// Tweet is one post in a timeline.
type Tweet struct {
	ID             string
	Text           string
	CreatedAt      time.Time
	Author         User
	ConversationID string
	Metrics        Metrics
	Media          []Media
	Refs           []Ref
	Liked          bool
	Reposted       bool
	Bookmarked     bool
	Hidden         bool
	Sensitive      bool
	IsRepost       bool
	RepostedBy     User
}

// ActionID is the post that like, repost, and reply act on.
// A repost row acts on the original post.
func (t Tweet) ActionID() string {
	if t.IsRepost {
		for _, r := range t.Refs {
			if r.Type == "retweeted" && r.ID != "" {
				return r.ID
			}
		}
	}
	return t.ID
}

// URL is the public permalink.
func (t Tweet) URL() string {
	user := t.Author.Username
	if user == "" {
		user = "i"
	}
	return "https://x.com/" + user + "/status/" + t.ActionID()
}

// ReplyTo is the handle this post replies to, without @.
func (t Tweet) ReplyTo() string {
	for _, r := range t.Refs {
		if r.Type == "replied_to" {
			return r.Author
		}
	}
	return ""
}

// Quote is the quoted post, if any.
func (t Tweet) Quote() (Ref, bool) {
	for _, r := range t.Refs {
		if r.Type == "quoted" {
			return r, true
		}
	}
	return Ref{}, false
}

// Timeline is one page of posts.
type Timeline struct {
	Tweets    []Tweet
	NextToken string
}

// List is an X list the user can open.
type List struct {
	ID          string
	Name        string
	Description string
	Members     int
	Owner       string
}

// Feed selects a timeline endpoint.
type Feed int

const (
	FeedHome Feed = iota
	FeedMentions
	FeedBookmarks
	FeedLikes
	FeedUser
	FeedList
	FeedSearch
)
