package xapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Scopes are the permissions xTui asks for. Register the same set on the app.
var Scopes = []string{
	"tweet.read",
	"tweet.write",
	"users.read",
	"like.read",
	"like.write",
	"bookmark.read",
	"bookmark.write",
	"follows.read",
	"follows.write",
	"mute.read",
	"mute.write",
	"block.read",
	"block.write",
	"list.read",
	"offline.access",
	"tweet.moderate.write",
}

const (
	authorizeEndpoint = "https://x.com/i/oauth2/authorize"
	defaultTokenURL   = "https://api.x.com/2/oauth2/token"
)

// Token is an OAuth 2.0 user token.
type Token struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
	Scope        string
}

// ChallengeS256 is the PKCE S256 challenge for verifier.
func ChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// NewPKCE returns a verifier and its S256 challenge.
func NewPKCE() (verifier, challenge string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf)
	return verifier, ChallengeS256(verifier), nil
}

// AuthorizeURL builds the browser sign-in URL.
func AuthorizeURL(clientID, redirectURI, state, challenge string) string {
	v := url.Values{}
	v.Set("response_type", "code")
	v.Set("client_id", clientID)
	v.Set("redirect_uri", redirectURI)
	v.Set("scope", strings.Join(Scopes, " "))
	v.Set("state", state)
	v.Set("code_challenge", challenge)
	v.Set("code_challenge_method", "S256")
	return authorizeEndpoint + "?" + v.Encode()
}

// Authorize opens the browser, waits for the loopback callback, and exchanges
// the code. port is the loopback port. The redirect URI is
// http://127.0.0.1:<port>/callback and must match the X app settings.
func Authorize(ctx context.Context, clientID, clientSecret string, port int, started func(authURL string)) (Token, error) {
	if port == 0 {
		port = 53682
	}
	verifier, challenge, err := NewPKCE()
	if err != nil {
		return Token{}, err
	}
	state, err := randomState()
	if err != nil {
		return Token{}, err
	}
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)
	authURL := AuthorizeURL(clientID, redirectURI, state, challenge)

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return Token{}, fmt.Errorf("listen on %s: %w", redirectURI, err)
	}
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(state)) != 1 {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			errCh <- fmt.Errorf("oauth state mismatch")
			return
		}
		if e := r.URL.Query().Get("error"); e != "" {
			desc := r.URL.Query().Get("error_description")
			http.Error(w, e, http.StatusBadRequest)
			if desc != "" {
				errCh <- fmt.Errorf("oauth: %s: %s", e, desc)
				return
			}
			errCh <- fmt.Errorf("oauth: %s", e)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- fmt.Errorf("oauth callback missing code")
			return
		}
		_, _ = io.WriteString(w, "xTui is signed in. You can close this tab.\n")
		codeCh <- code
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	if started != nil {
		started(authURL)
	}
	if err := openBrowser(authURL); err != nil {
		return Token{}, fmt.Errorf("the browser did not open. Visit this URL yourself: %s", authURL)
	}
	select {
	case <-ctx.Done():
		return Token{}, ctx.Err()
	case err := <-errCh:
		return Token{}, err
	case code := <-codeCh:
		return exchangeCode(ctx, http.DefaultClient, defaultTokenURL, clientID, clientSecret, code, verifier, redirectURI)
	}
}

func randomState() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func exchangeCode(ctx context.Context, client *http.Client, tokenURL, clientID, clientSecret, code, verifier, redirectURI string) (Token, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", verifier)
	form.Set("client_id", clientID)
	return tokenRequest(ctx, client, tokenURL, clientID, clientSecret, form)
}

func refreshRequest(ctx context.Context, client *http.Client, tokenURL, clientID, clientSecret, refreshToken string) (Token, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", clientID)
	return tokenRequest(ctx, client, tokenURL, clientID, clientSecret, form)
}

func tokenRequest(ctx context.Context, client *http.Client, tokenURL, clientID, clientSecret string, form url.Values) (Token, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if clientSecret != "" {
		req.SetBasicAuth(clientID, clientSecret)
	}
	res, err := client.Do(req)
	if err != nil {
		return Token{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return Token{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Token{}, fmt.Errorf("token endpoint: %s", apiDetail(res.StatusCode, body))
	}
	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Token{}, fmt.Errorf("decode token: %w", err)
	}
	if raw.AccessToken == "" {
		if raw.ErrorDesc != "" {
			return Token{}, fmt.Errorf("token endpoint: %s", raw.ErrorDesc)
		}
		if raw.Error != "" {
			return Token{}, fmt.Errorf("token endpoint: %s", raw.Error)
		}
		return Token{}, fmt.Errorf("token endpoint returned no access token")
	}
	tok := Token{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		Scope:        raw.Scope,
	}
	if raw.ExpiresIn > 0 {
		tok.Expiry = time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second)
	}
	return tok, nil
}

func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}
