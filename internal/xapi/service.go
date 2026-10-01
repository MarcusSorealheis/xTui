package xapi

import "context"

// Service is the set of calls the terminal uses. The HTTP client and the
// demo feed both implement it.
type Service interface {
	Me(ctx context.Context) (User, error)
	Timeline(ctx context.Context, feed Feed, arg, page string) (Timeline, error)
	Lists(ctx context.Context) ([]List, error)
	Thread(ctx context.Context, tweet Tweet) ([]Tweet, error)
	Like(ctx context.Context, tweetID string) error
	Unlike(ctx context.Context, tweetID string) error
	Repost(ctx context.Context, tweetID string) error
	Unrepost(ctx context.Context, tweetID string) error
	Bookmark(ctx context.Context, tweetID string) error
	Unbookmark(ctx context.Context, tweetID string) error
	Follow(ctx context.Context, userID string) error
	Unfollow(ctx context.Context, userID string) error
	Mute(ctx context.Context, userID string) error
	Unmute(ctx context.Context, userID string) error
	Block(ctx context.Context, userID string) error
	Unblock(ctx context.Context, userID string) error
	HideReply(ctx context.Context, tweetID string, hidden bool) error
	Post(ctx context.Context, text, replyTo, quoteID string) (Tweet, error)
	Delete(ctx context.Context, tweetID string) error
	Rate() string
}
