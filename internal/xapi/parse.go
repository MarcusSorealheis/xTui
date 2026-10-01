package xapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type rawUser struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Username         string   `json:"username"`
	Description      string   `json:"description"`
	Verified         bool     `json:"verified"`
	ConnectionStatus []string `json:"connection_status"`
}

type rawTweet struct {
	ID                string `json:"id"`
	Text              string `json:"text"`
	CreatedAt         string `json:"created_at"`
	AuthorID          string `json:"author_id"`
	ConversationID    string `json:"conversation_id"`
	PossiblySensitive bool   `json:"possibly_sensitive"`
	NoteTweet         struct {
		Text string `json:"text"`
	} `json:"note_tweet"`
	PublicMetrics struct {
		RetweetCount  int `json:"retweet_count"`
		ReplyCount    int `json:"reply_count"`
		LikeCount     int `json:"like_count"`
		QuoteCount    int `json:"quote_count"`
		BookmarkCount int `json:"bookmark_count"`
	} `json:"public_metrics"`
	Entities struct {
		URLs []struct {
			URL      string `json:"url"`
			Expanded string `json:"expanded_url"`
			Display  string `json:"display_url"`
		} `json:"urls"`
	} `json:"entities"`
	Attachments struct {
		MediaKeys []string `json:"media_keys"`
	} `json:"attachments"`
	ReferencedTweets []struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	} `json:"referenced_tweets"`
}

type rawMedia struct {
	MediaKey string `json:"media_key"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	Preview  string `json:"preview_image_url"`
	Alt      string `json:"alt_text"`
}

type envelope struct {
	Data     json.RawMessage `json:"data"`
	Includes struct {
		Users  []rawUser  `json:"users"`
		Tweets []rawTweet `json:"tweets"`
		Media  []rawMedia `json:"media"`
	} `json:"includes"`
	Meta struct {
		NextToken   string `json:"next_token"`
		ResultCount int    `json:"result_count"`
	} `json:"meta"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func parseTimeline(body []byte) (Timeline, error) {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return Timeline{}, fmt.Errorf("decode timeline: %w", err)
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		if msg := env.errorMessage(); msg != "" && env.Meta.ResultCount == 0 {
			return Timeline{}, fmt.Errorf("%s", msg)
		}
		return Timeline{NextToken: env.Meta.NextToken}, nil
	}
	raws, err := decodeTweets(env.Data)
	if err != nil {
		return Timeline{}, err
	}
	users := map[string]rawUser{}
	for _, u := range env.Includes.Users {
		users[u.ID] = u
	}
	included := map[string]rawTweet{}
	for _, tw := range env.Includes.Tweets {
		included[tw.ID] = tw
	}
	media := map[string]rawMedia{}
	for _, m := range env.Includes.Media {
		media[m.MediaKey] = m
	}
	out := make([]Tweet, 0, len(raws))
	for _, raw := range raws {
		out = append(out, buildTweet(raw, users, included, media))
	}
	return Timeline{Tweets: out, NextToken: env.Meta.NextToken}, nil
}

func decodeTweets(raw json.RawMessage) ([]rawTweet, error) {
	trim := strings.TrimSpace(string(raw))
	if trim == "" || trim == "null" {
		return nil, nil
	}
	if strings.HasPrefix(trim, "[") {
		var tweets []rawTweet
		if err := json.Unmarshal(raw, &tweets); err != nil {
			return nil, fmt.Errorf("decode tweets: %w", err)
		}
		return tweets, nil
	}
	var one rawTweet
	if err := json.Unmarshal(raw, &one); err != nil {
		return nil, fmt.Errorf("decode tweet: %w", err)
	}
	return []rawTweet{one}, nil
}

func buildTweet(raw rawTweet, users map[string]rawUser, included map[string]rawTweet, media map[string]rawMedia) Tweet {
	tw := tweetFromRaw(raw, users, media)
	for _, ref := range raw.ReferencedTweets {
		r := Ref{Type: ref.Type, ID: ref.ID}
		if orig, ok := included[ref.ID]; ok {
			r.Text = displayText(orig)
			if u, ok := users[orig.AuthorID]; ok {
				r.Author = u.Username
			}
			if ref.Type == "retweeted" {
				origTweet := tweetFromRaw(orig, users, media)
				origTweet.ID = raw.ID
				origTweet.IsRepost = true
				origTweet.RepostedBy = tw.Author
				origTweet.Refs = append(origTweet.Refs, r)
				return origTweet
			}
		}
		tw.Refs = append(tw.Refs, r)
	}
	return tw
}

func tweetFromRaw(raw rawTweet, users map[string]rawUser, media map[string]rawMedia) Tweet {
	tw := Tweet{
		ID:             raw.ID,
		Text:           displayText(raw),
		Author:         userFromRaw(users[raw.AuthorID]),
		ConversationID: raw.ConversationID,
		Sensitive:      raw.PossiblySensitive,
		Metrics: Metrics{
			Likes:     raw.PublicMetrics.LikeCount,
			Reposts:   raw.PublicMetrics.RetweetCount,
			Replies:   raw.PublicMetrics.ReplyCount,
			Quotes:    raw.PublicMetrics.QuoteCount,
			Bookmarks: raw.PublicMetrics.BookmarkCount,
		},
	}
	if raw.AuthorID != "" && tw.Author.ID == "" {
		tw.Author.ID = raw.AuthorID
	}
	if raw.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, raw.CreatedAt); err == nil {
			tw.CreatedAt = t
		}
	}
	for _, key := range raw.Attachments.MediaKeys {
		if m, ok := media[key]; ok {
			url := m.URL
			if url == "" {
				url = m.Preview
			}
			tw.Media = append(tw.Media, Media{Type: m.Type, URL: url, Alt: m.Alt})
		}
	}
	return tw
}

func displayText(raw rawTweet) string {
	text := raw.Text
	if note := strings.TrimSpace(raw.NoteTweet.Text); note != "" {
		text = note
	}
	for _, u := range raw.Entities.URLs {
		if u.URL == "" || u.Display == "" {
			continue
		}
		text = strings.ReplaceAll(text, u.URL, u.Display)
	}
	return text
}

func userFromRaw(raw rawUser) User {
	u := User{
		ID:          raw.ID,
		Name:        raw.Name,
		Username:    raw.Username,
		Description: raw.Description,
		Verified:    raw.Verified,
	}
	for _, status := range raw.ConnectionStatus {
		switch status {
		case "following":
			u.Following = true
		case "followed_by":
			u.FollowedBy = true
		case "muting":
			u.Muting = true
		case "blocking":
			u.Blocking = true
		}
	}
	return u
}

func (e envelope) errorMessage() string {
	if e.Detail != "" {
		return e.Detail
	}
	if e.Title != "" && len(e.Errors) == 0 {
		return e.Title
	}
	if len(e.Errors) > 0 && e.Errors[0].Message != "" {
		return e.Errors[0].Message
	}
	return e.Title
}

type rawList struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MemberCount int    `json:"member_count"`
	OwnerID     string `json:"owner_id"`
}

func parseLists(body []byte) ([]List, error) {
	var env struct {
		Data     []rawList `json:"data"`
		Includes struct {
			Users []rawUser `json:"users"`
		} `json:"includes"`
		Title  string `json:"title"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode lists: %w", err)
	}
	if env.Detail != "" && len(env.Data) == 0 {
		return nil, fmt.Errorf("%s", env.Detail)
	}
	owners := map[string]string{}
	for _, u := range env.Includes.Users {
		owners[u.ID] = u.Username
	}
	out := make([]List, 0, len(env.Data))
	for _, raw := range env.Data {
		out = append(out, List{
			ID:          raw.ID,
			Name:        raw.Name,
			Description: raw.Description,
			Members:     raw.MemberCount,
			Owner:       owners[raw.OwnerID],
		})
	}
	return out, nil
}
