package xapi

import (
	"strings"
	"testing"
	"time"
)

func TestParseTimelineRetweetQuoteAndNote(t *testing.T) {
	body := []byte(`{
	  "data": [
	    {
	      "id": "10",
	      "text": "RT short",
	      "created_at": "2026-09-30T15:04:05Z",
	      "author_id": "2",
	      "referenced_tweets": [{"type": "retweeted", "id": "9"}]
	    },
	    {
	      "id": "11",
	      "text": "see https://t.co/abc",
	      "author_id": "3",
	      "note_tweet": {"text": "the long form of the post, with the link https://t.co/abc kept readable"},
	      "public_metrics": {"like_count": 4, "retweet_count": 1, "reply_count": 2, "quote_count": 1, "bookmark_count": 3},
	      "entities": {"urls": [{"url": "https://t.co/abc", "expanded_url": "https://example.com/post", "display_url": "example.com/post"}]},
	      "attachments": {"media_keys": ["m1"]},
	      "referenced_tweets": [{"type": "quoted", "id": "8"}, {"type": "replied_to", "id": "7"}],
	      "conversation_id": "7"
	    }
	  ],
	  "includes": {
	    "users": [
	      {"id": "2", "name": "Reposter", "username": "reposter"},
	      {"id": "1", "name": "Author", "username": "author", "verified": true, "connection_status": ["following"]},
	      {"id": "3", "name": "Quoter", "username": "quoter", "connection_status": ["muting"]}
	    ],
	    "tweets": [
	      {"id": "9", "text": "original post", "author_id": "1", "created_at": "2026-09-30T15:00:00Z"},
	      {"id": "8", "text": "quoted text", "author_id": "1"},
	      {"id": "7", "text": "parent", "author_id": "1"}
	    ],
	    "media": [{"media_key": "m1", "type": "photo", "url": "https://img.example/p.jpg", "alt_text": "a diagram"}]
	  },
	  "meta": {"result_count": 2, "next_token": "NEXT"}
	}`)
	tl, err := parseTimeline(body)
	if err != nil {
		t.Fatal(err)
	}
	if tl.NextToken != "NEXT" || len(tl.Tweets) != 2 {
		t.Fatalf("timeline = %+v", tl)
	}
	rt := tl.Tweets[0]
	if !rt.IsRepost || rt.Author.Username != "author" || rt.RepostedBy.Username != "reposter" || rt.Text != "original post" {
		t.Fatalf("retweet = %+v", rt)
	}
	if !rt.Author.Verified || !rt.Author.Following {
		t.Fatalf("author flags = %+v", rt.Author)
	}
	if rt.ActionID() != "9" || !strings.Contains(rt.URL(), "/author/status/9") {
		t.Fatalf("action id/url = %s %s", rt.ActionID(), rt.URL())
	}
	q := tl.Tweets[1]
	if !strings.Contains(q.Text, "long form") || !strings.Contains(q.Text, "example.com/post") {
		t.Fatalf("text = %q", q.Text)
	}
	if q.Metrics.Likes != 4 || q.Metrics.Bookmarks != 3 || len(q.Media) != 1 || q.Media[0].Alt != "a diagram" {
		t.Fatalf("metrics/media = %+v %+v", q.Metrics, q.Media)
	}
	if q.Author.Muting != true || q.ReplyTo() != "author" {
		t.Fatalf("user/reply = %+v reply-to %q", q.Author, q.ReplyTo())
	}
	quote, ok := q.Quote()
	if !ok || quote.Text != "quoted text" {
		t.Fatalf("quote = %+v", quote)
	}
	if rt.CreatedAt != time.Date(2026, 9, 30, 15, 0, 0, 0, time.UTC) {
		t.Fatal(rt.CreatedAt)
	}
}

func TestParseEmptyAndLists(t *testing.T) {
	tl, err := parseTimeline([]byte(`{"meta":{"result_count":0}}`))
	if err != nil || len(tl.Tweets) != 0 {
		t.Fatalf("empty = %+v %v", tl, err)
	}
	lists, err := parseLists([]byte(`{
	  "data": [{"id": "5", "name": "builds", "description": "ci", "member_count": 3, "owner_id": "1"}],
	  "includes": {"users": [{"id": "1", "username": "ada"}]}
	}`))
	if err != nil || len(lists) != 1 || lists[0].Owner != "ada" || lists[0].Members != 3 {
		t.Fatalf("lists = %+v %v", lists, err)
	}
}

func TestParseSingleTweetObject(t *testing.T) {
	tl, err := parseTimeline([]byte(`{"data":{"id":"1","text":"solo","author_id":"2"},"includes":{"users":[{"id":"2","username":"g"}]}}`))
	if err != nil || len(tl.Tweets) != 1 || tl.Tweets[0].Author.Username != "g" {
		t.Fatalf("%+v %v", tl, err)
	}
}
