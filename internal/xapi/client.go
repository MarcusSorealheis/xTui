package xapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultAPIBase = "https://api.x.com"

// Options configures an API client.
type Options struct {
	BaseURL      string
	TokenURL     string
	HTTP         *http.Client
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
	ClientID     string
	ClientSecret string
	UserID       string
	Persist      func(Token)
}

// Client is an OAuth 2.0 user-context X API client.
type Client struct {
	base         string
	tokenURL     string
	http         *http.Client
	clientID     string
	clientSecret string
	persist      func(Token)

	mu         sync.Mutex
	access     string
	refresh    string
	expiry     time.Time
	userID     string
	safeFields bool
	rate       string
}

// New returns a client. It does not call the network.
func New(opts Options) *Client {
	base := strings.TrimRight(opts.BaseURL, "/")
	if base == "" {
		base = defaultAPIBase
	}
	tokenURL := opts.TokenURL
	if tokenURL == "" {
		tokenURL = defaultTokenURL
	}
	hc := opts.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		base:         base,
		tokenURL:     tokenURL,
		http:         hc,
		clientID:     opts.ClientID,
		clientSecret: opts.ClientSecret,
		persist:      opts.Persist,
		access:       opts.AccessToken,
		refresh:      opts.RefreshToken,
		expiry:       opts.Expiry,
		userID:       opts.UserID,
	}
}

// Rate is the last X-Rate-Limit-Remaining value seen, if the API sent one.
func (c *Client) Rate() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.rate
}

// Me returns the signed-in user.
func (c *Client) Me(ctx context.Context) (User, error) {
	q := url.Values{}
	q.Set("user.fields", "id,name,username,description,verified,connection_status")
	body, _, err := c.get(ctx, "/2/users/me", q)
	if err != nil && c.useSafeFields(err) {
		q.Set("user.fields", "id,name,username,description,verified")
		body, _, err = c.get(ctx, "/2/users/me", q)
	}
	if err != nil {
		return User{}, err
	}
	var env struct {
		Data rawUser `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return User{}, err
	}
	if env.Data.ID == "" {
		return User{}, fmt.Errorf("users/me returned no user")
	}
	user := userFromRaw(env.Data)
	c.mu.Lock()
	c.userID = user.ID
	c.mu.Unlock()
	return user, nil
}

// Timeline loads one page of a feed. arg is a user id, list id, or search query.
func (c *Client) Timeline(ctx context.Context, feed Feed, arg, page string) (Timeline, error) {
	path, queryName, err := c.feedPath(ctx, feed, arg)
	if err != nil {
		return Timeline{}, err
	}
	q := url.Values{}
	if page != "" {
		q.Set(queryName, page)
	}
	if feed == FeedSearch {
		q.Set("query", arg)
	}
	return c.getTweets(ctx, path, q, true)
}

func (c *Client) feedPath(ctx context.Context, feed Feed, arg string) (path, pageParam string, err error) {
	pageParam = "pagination_token"
	switch feed {
	case FeedHome:
		id, err := c.meID(ctx)
		if err != nil {
			return "", "", err
		}
		return "/2/users/" + url.PathEscape(id) + "/timelines/reverse_chronological", pageParam, nil
	case FeedMentions:
		id, err := c.meID(ctx)
		if err != nil {
			return "", "", err
		}
		return "/2/users/" + url.PathEscape(id) + "/mentions", pageParam, nil
	case FeedBookmarks:
		id, err := c.meID(ctx)
		if err != nil {
			return "", "", err
		}
		return "/2/users/" + url.PathEscape(id) + "/bookmarks", pageParam, nil
	case FeedLikes:
		id, err := c.meID(ctx)
		if err != nil {
			return "", "", err
		}
		return "/2/users/" + url.PathEscape(id) + "/liked_tweets", pageParam, nil
	case FeedUser:
		if arg == "" {
			arg, err = c.meID(ctx)
			if err != nil {
				return "", "", err
			}
		}
		return "/2/users/" + url.PathEscape(arg) + "/tweets", pageParam, nil
	case FeedList:
		if arg == "" {
			return "", "", fmt.Errorf("missing list id")
		}
		return "/2/lists/" + url.PathEscape(arg) + "/tweets", pageParam, nil
	case FeedSearch:
		if strings.TrimSpace(arg) == "" {
			return "", "", fmt.Errorf("missing search query")
		}
		return "/2/tweets/search/recent", "next_token", nil
	default:
		return "", "", fmt.Errorf("unknown feed")
	}
}

// Lists returns followed and owned lists, deduped by id.
func (c *Client) Lists(ctx context.Context) ([]List, error) {
	id, err := c.meID(ctx)
	if err != nil {
		return nil, err
	}
	followed, ferr := c.listPage(ctx, "/2/users/"+url.PathEscape(id)+"/followed_lists")
	owned, oerr := c.listPage(ctx, "/2/users/"+url.PathEscape(id)+"/owned_lists")
	if ferr != nil && oerr != nil {
		return nil, ferr
	}
	seen := map[string]bool{}
	var out []List
	for _, list := range append(owned, followed...) {
		if list.ID == "" || seen[list.ID] {
			continue
		}
		seen[list.ID] = true
		out = append(out, list)
	}
	return out, nil
}

func (c *Client) listPage(ctx context.Context, path string) ([]List, error) {
	q := url.Values{}
	q.Set("list.fields", "id,name,description,member_count,owner_id")
	q.Set("expansions", "owner_id")
	q.Set("user.fields", "username")
	q.Set("max_results", "100")
	body, _, err := c.get(ctx, path, q)
	if err != nil {
		return nil, err
	}
	return parseLists(body)
}

// Thread loads the conversation for tweet. Recent search covers about seven days.
func (c *Client) Thread(ctx context.Context, tweet Tweet) ([]Tweet, error) {
	conv := tweet.ConversationID
	if conv == "" {
		conv = tweet.ActionID()
	}
	tl, searchErr := c.Timeline(ctx, FeedSearch, "conversation_id:"+conv, "")
	root, rootErr := c.lookup(ctx, conv)
	out := []Tweet{}
	seen := map[string]bool{}
	add := func(t Tweet) {
		if t.ID == "" || seen[t.ID] {
			return
		}
		seen[t.ID] = true
		out = append(out, t)
	}
	if rootErr == nil {
		add(root)
	} else {
		add(tweet)
	}
	for _, t := range tl.Tweets {
		add(t)
	}
	sortByTime(out)
	if searchErr != nil && len(tl.Tweets) == 0 {
		return out, searchErr
	}
	return out, nil
}

func (c *Client) lookup(ctx context.Context, id string) (Tweet, error) {
	tl, err := c.getTweets(ctx, "/2/tweets/"+url.PathEscape(id), url.Values{}, false)
	if err != nil {
		return Tweet{}, err
	}
	if len(tl.Tweets) == 0 {
		return Tweet{}, fmt.Errorf("tweet %s not found", id)
	}
	return tl.Tweets[0], nil
}

// Like likes a post.
func (c *Client) Like(ctx context.Context, tweetID string) error {
	return c.userPost(ctx, "likes", map[string]string{"tweet_id": tweetID})
}

// Unlike removes a like.
func (c *Client) Unlike(ctx context.Context, tweetID string) error {
	return c.userDelete(ctx, "likes/"+url.PathEscape(tweetID))
}

// Repost reposts a post.
func (c *Client) Repost(ctx context.Context, tweetID string) error {
	return c.userPost(ctx, "retweets", map[string]string{"tweet_id": tweetID})
}

// Unrepost removes a repost.
func (c *Client) Unrepost(ctx context.Context, tweetID string) error {
	return c.userDelete(ctx, "retweets/"+url.PathEscape(tweetID))
}

// Bookmark saves a post.
func (c *Client) Bookmark(ctx context.Context, tweetID string) error {
	return c.userPost(ctx, "bookmarks", map[string]string{"tweet_id": tweetID})
}

// Unbookmark removes a bookmark.
func (c *Client) Unbookmark(ctx context.Context, tweetID string) error {
	return c.userDelete(ctx, "bookmarks/"+url.PathEscape(tweetID))
}

// Follow follows a user.
func (c *Client) Follow(ctx context.Context, userID string) error {
	return c.userPost(ctx, "following", map[string]string{"target_user_id": userID})
}

// Unfollow unfollows a user.
func (c *Client) Unfollow(ctx context.Context, userID string) error {
	return c.userDelete(ctx, "following/"+url.PathEscape(userID))
}

// Mute mutes a user.
func (c *Client) Mute(ctx context.Context, userID string) error {
	return c.userPost(ctx, "muting", map[string]string{"target_user_id": userID})
}

// Unmute unmutes a user.
func (c *Client) Unmute(ctx context.Context, userID string) error {
	return c.userDelete(ctx, "muting/"+url.PathEscape(userID))
}

// Block blocks a user.
func (c *Client) Block(ctx context.Context, userID string) error {
	return c.userPost(ctx, "blocking", map[string]string{"target_user_id": userID})
}

// Unblock unblocks a user.
func (c *Client) Unblock(ctx context.Context, userID string) error {
	return c.userDelete(ctx, "blocking/"+url.PathEscape(userID))
}

// HideReply hides or unhides a reply to one of your posts.
func (c *Client) HideReply(ctx context.Context, tweetID string, hidden bool) error {
	_, err := c.doJSON(ctx, http.MethodPut, "/2/tweets/"+url.PathEscape(tweetID)+"/hidden", map[string]bool{"hidden": hidden}, nil)
	return err
}

// Post publishes a post, a reply, or a quote.
func (c *Client) Post(ctx context.Context, text, replyTo, quoteID string) (Tweet, error) {
	payload := map[string]any{"text": text}
	if replyTo != "" {
		payload["reply"] = map[string]string{"in_reply_to_tweet_id": replyTo}
	}
	if quoteID != "" {
		payload["quote_tweet_id"] = quoteID
	}
	body, err := c.doJSON(ctx, http.MethodPost, "/2/tweets", payload, nil)
	if err != nil {
		return Tweet{}, err
	}
	tl, err := parseTimeline(body)
	if err != nil {
		return Tweet{}, err
	}
	if len(tl.Tweets) == 0 {
		return Tweet{Text: text}, nil
	}
	tw := tl.Tweets[0]
	if tw.Text == "" {
		tw.Text = text
	}
	return tw, nil
}

// Delete deletes one of your posts.
func (c *Client) Delete(ctx context.Context, tweetID string) error {
	_, _, err := c.do(ctx, http.MethodDelete, "/2/tweets/"+url.PathEscape(tweetID), nil, nil, true)
	return err
}

func (c *Client) userPost(ctx context.Context, leaf string, payload any) error {
	id, err := c.meID(ctx)
	if err != nil {
		return err
	}
	_, err = c.doJSON(ctx, http.MethodPost, "/2/users/"+url.PathEscape(id)+"/"+leaf, payload, nil)
	return err
}

func (c *Client) userDelete(ctx context.Context, leaf string) error {
	id, err := c.meID(ctx)
	if err != nil {
		return err
	}
	_, _, err = c.do(ctx, http.MethodDelete, "/2/users/"+url.PathEscape(id)+"/"+leaf, nil, nil, true)
	return err
}

func (c *Client) getTweets(ctx context.Context, path string, q url.Values, paged bool) (Timeline, error) {
	c.applyTweetFields(q, paged)
	body, _, err := c.get(ctx, path, q)
	if err != nil && c.useSafeFields(err) {
		c.applyTweetFields(q, paged)
		body, _, err = c.get(ctx, path, q)
	}
	if err != nil {
		return Timeline{}, err
	}
	return parseTimeline(body)
}

func (c *Client) applyTweetFields(q url.Values, paged bool) {
	c.mu.Lock()
	safe := c.safeFields
	c.mu.Unlock()
	if safe {
		q.Set("tweet.fields", "id,text,created_at,author_id,conversation_id,public_metrics,entities,attachments,referenced_tweets,possibly_sensitive")
		q.Set("user.fields", "id,name,username,verified,description")
	} else {
		q.Set("tweet.fields", "id,text,created_at,author_id,conversation_id,public_metrics,entities,attachments,referenced_tweets,possibly_sensitive,note_tweet")
		q.Set("user.fields", "id,name,username,verified,description,connection_status")
	}
	q.Set("expansions", "author_id,referenced_tweets.id,referenced_tweets.id.author_id,attachments.media_keys")
	q.Set("media.fields", "media_key,type,url,preview_image_url,alt_text")
	if paged && q.Get("max_results") == "" {
		q.Set("max_results", "20")
	}
}

func (c *Client) useSafeFields(err error) bool {
	msg := err.Error()
	if !strings.Contains(msg, "note_tweet") && !strings.Contains(msg, "connection_status") {
		return false
	}
	c.mu.Lock()
	already := c.safeFields
	c.safeFields = true
	c.mu.Unlock()
	return !already
}

func (c *Client) get(ctx context.Context, path string, q url.Values) ([]byte, int, error) {
	return c.do(ctx, http.MethodGet, path, q, nil, true)
}

func (c *Client) doJSON(ctx context.Context, method, path string, payload any, q url.Values) ([]byte, error) {
	var buf io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewReader(b)
	}
	body, _, err := c.do(ctx, method, path, q, buf, true)
	return body, err
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, body io.Reader, retryAuth bool) ([]byte, int, error) {
	var snap []byte
	if body != nil {
		var err error
		snap, err = io.ReadAll(body)
		if err != nil {
			return nil, 0, err
		}
	}
	token, err := c.bearer(ctx)
	if err != nil {
		return nil, 0, err
	}
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(snap))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "xTui/0.1")
	if method != http.MethodGet && method != http.MethodDelete {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	resp, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, res.StatusCode, err
	}
	c.noteRate(res.Header.Get("X-Rate-Limit-Remaining"))
	if res.StatusCode == http.StatusUnauthorized && retryAuth && c.hasRefresh() {
		if rerr := c.refreshToken(ctx); rerr == nil {
			return c.do(ctx, method, path, q, bytes.NewReader(snap), false)
		}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, res.StatusCode, fmt.Errorf("%s", apiDetail(res.StatusCode, resp))
	}
	return resp, res.StatusCode, nil
}

func (c *Client) noteRate(remaining string) {
	if remaining == "" {
		return
	}
	c.mu.Lock()
	c.rate = remaining
	c.mu.Unlock()
}

func (c *Client) hasRefresh() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.refresh != ""
}

func (c *Client) bearer(ctx context.Context) (string, error) {
	c.mu.Lock()
	access, refresh, expiry := c.access, c.refresh, c.expiry
	c.mu.Unlock()
	fresh := access != "" && (expiry.IsZero() || time.Now().Before(expiry.Add(-45*time.Second)))
	if fresh {
		return access, nil
	}
	if refresh == "" {
		if access != "" && (expiry.IsZero() || time.Now().Before(expiry)) {
			return access, nil
		}
		return "", fmt.Errorf("sign in required")
	}
	if err := c.refreshToken(ctx); err != nil {
		if access != "" && (expiry.IsZero() || time.Now().Before(expiry)) {
			return access, nil
		}
		return "", err
	}
	c.mu.Lock()
	access = c.access
	c.mu.Unlock()
	if access == "" {
		return "", fmt.Errorf("sign in required")
	}
	return access, nil
}

func (c *Client) refreshToken(ctx context.Context) error {
	c.mu.Lock()
	refresh := c.refresh
	id := c.clientID
	secret := c.clientSecret
	tokenURL := c.tokenURL
	c.mu.Unlock()
	if refresh == "" {
		return fmt.Errorf("session expired")
	}
	tok, err := refreshRequest(ctx, c.http, tokenURL, id, secret, refresh)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.access = tok.AccessToken
	if tok.RefreshToken != "" {
		c.refresh = tok.RefreshToken
	}
	c.expiry = tok.Expiry
	c.mu.Unlock()
	if c.persist != nil {
		c.persist(tok)
	}
	return nil
}

func (c *Client) meID(ctx context.Context) (string, error) {
	c.mu.Lock()
	id := c.userID
	c.mu.Unlock()
	if id != "" {
		return id, nil
	}
	me, err := c.Me(ctx)
	if err != nil {
		return "", err
	}
	return me.ID, nil
}

func apiDetail(status int, body []byte) string {
	var env envelope
	if json.Unmarshal(body, &env) == nil {
		if msg := env.errorMessage(); msg != "" {
			return msg
		}
	}
	text := strings.TrimSpace(string(body))
	if len(text) > 180 {
		text = text[:180]
	}
	if text == "" {
		return fmt.Sprintf("x api status %d", status)
	}
	return text
}

func sortByTime(tweets []Tweet) {
	for i := 1; i < len(tweets); i++ {
		j := i
		for j > 0 && tweets[j].CreatedAt.Before(tweets[j-1].CreatedAt) {
			tweets[j], tweets[j-1] = tweets[j-1], tweets[j]
			j--
		}
	}
}
