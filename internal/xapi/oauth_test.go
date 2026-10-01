package xapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPKCEVector(t *testing.T) {
	// RFC 7636 appendix B.
	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	const want = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got := ChallengeS256(verifier); got != want {
		t.Fatalf("challenge = %s", got)
	}
	v, c, err := NewPKCE()
	if err != nil {
		t.Fatal(err)
	}
	if ChallengeS256(v) != c || len(v) < 43 {
		t.Fatalf("verifier %q challenge %q", v, c)
	}
}

func TestAuthorizeURL(t *testing.T) {
	u := AuthorizeURL("client", "http://127.0.0.1:53682/callback", "state", "challenge")
	if !strings.Contains(u, "code_challenge_method=S256") || !strings.Contains(u, "offline.access") {
		t.Fatal(u)
	}
	if !strings.HasPrefix(u, "https://x.com/i/oauth2/authorize?") {
		t.Fatal(u)
	}
}

func TestRefreshRequest(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		user, pass, ok := r.BasicAuth()
		if !ok || user != "cid" || pass != "sec" {
			t.Errorf("basic = %s %s %v", user, pass, ok)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"new","refresh_token":"newer","expires_in":7200,"scope":"tweet.read"}`)
	}))
	defer srv.Close()
	tok, err := refreshRequest(context.Background(), srv.Client(), srv.URL, "cid", "sec", "old")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "new" || tok.RefreshToken != "newer" || tok.Expiry.IsZero() {
		t.Fatalf("%+v", tok)
	}
	if !strings.Contains(got, "grant_type=refresh_token") || !strings.Contains(got, "client_id=cid") {
		t.Fatal(got)
	}
}
