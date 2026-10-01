package xapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientHomeLikeAndRefresh(t *testing.T) {
	var refreshed bool
	mux := http.NewServeMux()
	mux.HandleFunc("/2/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		refreshed = true
		_, _ = io.WriteString(w, `{"access_token":"second","refresh_token":"r2","expires_in":7200}`)
	})
	mux.HandleFunc("/2/users/me", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer second" && got != "Bearer first" {
			t.Errorf("auth %s", got)
		}
		_, _ = io.WriteString(w, `{"data":{"id":"42","name":"Ada","username":"ada","verified":true}}`)
	})
	mux.HandleFunc("/2/users/42/timelines/reverse_chronological", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pagination_token") != "NEXT" {
			t.Errorf("page %s", r.URL.Query().Get("pagination_token"))
		}
		if !strings.Contains(r.URL.Query().Get("tweet.fields"), "public_metrics") {
			t.Errorf("fields %s", r.URL.RawQuery)
		}
		w.Header().Set("X-Rate-Limit-Remaining", "17")
		_, _ = io.WriteString(w, `{
		  "data":[{"id":"7","text":"hello https://t.co/abc","author_id":"9","created_at":"2026-09-30T00:00:00Z",
		    "entities":{"urls":[{"url":"https://t.co/abc","display_url":"example.com","expanded_url":"https://example.com"}]}}],
		  "includes":{"users":[{"id":"9","username":"grace","name":"Grace"}]},
		  "meta":{"next_token":"MORE","result_count":1}
		}`)
	})
	mux.HandleFunc("/2/users/42/likes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), `"tweet_id":"7"`) {
			t.Errorf("body %s", b)
		}
		_, _ = io.WriteString(w, `{"data":{"liked":true}}`)
	})
	mux.HandleFunc("/2/users/42/likes/7", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method %s", r.Method)
		}
		_, _ = io.WriteString(w, `{"data":{"liked":false}}`)
	})
	mux.HandleFunc("/2/tweets", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), `"in_reply_to_tweet_id":"7"`) || !strings.Contains(string(b), `"text":"hi"`) {
			t.Errorf("body %s", b)
		}
		_, _ = io.WriteString(w, `{"data":{"id":"8","text":"hi"}}`)
	})
	mux.HandleFunc("/2/tweets/search/recent", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") != "conversation_id:7" {
			t.Errorf("query %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("pagination_token") != "" {
			t.Errorf("search should use next_token, got %s", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"data":[{"id":"7","text":"hello","author_id":"9"}],"includes":{"users":[{"id":"9","username":"grace"}]}}`)
	})
	mux.HandleFunc("/2/tweets/7", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			_, _ = io.WriteString(w, `{"data":{"deleted":true}}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"id":"7","text":"hello","author_id":"9","created_at":"2026-09-30T00:00:00Z"},"includes":{"users":[{"id":"9","username":"grace"}]}}`)
	})
	mux.HandleFunc("/2/tweets/7/hidden", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method %s", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		if string(b) != `{"hidden":true}` {
			t.Errorf("body %s", b)
		}
		_, _ = io.WriteString(w, `{"data":{"hidden":true}}`)
	})

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/2/users/me" {
			calls++
			if calls == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.WriteString(w, `{"title":"Unauthorized","detail":"expired"}`)
				return
			}
		}
		mux.ServeHTTP(w, r)
	}))
	defer srv.Close()

	var saved Token
	c := New(Options{
		BaseURL:      srv.URL,
		TokenURL:     srv.URL + "/2/oauth2/token",
		HTTP:         srv.Client(),
		AccessToken:  "first",
		RefreshToken: "old",
		Expiry:       time.Now().Add(-time.Hour),
		ClientID:     "cid",
		ClientSecret: "sec",
		Persist:      func(tok Token) { saved = tok },
	})
	me, err := c.Me(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if me.Username != "ada" || me.ID != "42" || !refreshed || saved.AccessToken != "second" {
		t.Fatalf("me %+v saved %+v refreshed %v", me, saved, refreshed)
	}
	tl, err := c.Timeline(context.Background(), FeedHome, "", "NEXT")
	if err != nil {
		t.Fatal(err)
	}
	if len(tl.Tweets) != 1 || tl.Tweets[0].Author.Username != "grace" || tl.NextToken != "MORE" {
		t.Fatalf("%+v", tl)
	}
	if !strings.Contains(tl.Tweets[0].Text, "example.com") {
		t.Fatal(tl.Tweets[0].Text)
	}
	if c.Rate() != "17" {
		t.Fatal(c.Rate())
	}
	if err := c.Like(context.Background(), "7"); err != nil {
		t.Fatal(err)
	}
	if err := c.Unlike(context.Background(), "7"); err != nil {
		t.Fatal(err)
	}
	posted, err := c.Post(context.Background(), "hi", "7", "")
	if err != nil || posted.ID != "8" {
		t.Fatalf("%+v %v", posted, err)
	}
	thread, err := c.Thread(context.Background(), Tweet{ID: "7", ConversationID: "7"})
	if err != nil || len(thread) == 0 {
		t.Fatalf("%+v %v", thread, err)
	}
	if err := c.HideReply(context.Background(), "7", true); err != nil {
		t.Fatal(err)
	}
	if err := c.Delete(context.Background(), "7"); err != nil {
		t.Fatal(err)
	}
}

func TestClientSurfacesAPIDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"title":"Forbidden","detail":"bookmark.read is not on this app"}`)
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL, HTTP: srv.Client(), AccessToken: "tok", UserID: "1"})
	_, err := c.Timeline(context.Background(), FeedBookmarks, "", "")
	if err == nil || !strings.Contains(err.Error(), "bookmark.read") {
		t.Fatal(err)
	}
}

func TestDemoMutations(t *testing.T) {
	d := NewDemo()
	ctx := context.Background()
	me, err := d.Me(ctx)
	if err != nil || me.Username != "ada" {
		t.Fatal(me, err)
	}
	home, err := d.Timeline(ctx, FeedHome, "", "")
	if err != nil || len(home.Tweets) != 4 || home.NextToken != "p2" {
		t.Fatalf("%+v %v", home, err)
	}
	rest, err := d.Timeline(ctx, FeedHome, "", "p2")
	if err != nil || len(rest.Tweets) != 3 || rest.NextToken != "" {
		t.Fatalf("rest %+v %v", rest, err)
	}
	if err := d.Like(ctx, home.Tweets[0].ActionID()); err != nil {
		t.Fatal(err)
	}
	likes, err := d.Timeline(ctx, FeedLikes, "", "")
	if err != nil || len(likes.Tweets) < 2 {
		t.Fatalf("likes %+v %v", likes, err)
	}
	posted, err := d.Post(ctx, "hello from the demo", home.Tweets[0].ID, "")
	if err != nil || posted.Author.Username != "ada" {
		t.Fatal(posted, err)
	}
	if err := d.Delete(ctx, posted.ID); err != nil {
		t.Fatal(err)
	}
	if err := d.Delete(ctx, "101"); err == nil {
		t.Fatal("deleted someone else's post")
	}
}
