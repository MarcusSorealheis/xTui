package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XTUI_CONFIG_DIR", dir)
	in := Config{
		ClientID:     "cid",
		ClientSecret: "sec",
		AccessToken:  "access",
		RefreshToken: "refresh",
		Expiry:       time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC),
		RedirectPort: 53682,
	}
	if err := Save(in); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
	out, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if out.AccessToken != in.AccessToken || out.ClientID != in.ClientID || !out.Expiry.Equal(in.Expiry) {
		t.Fatalf("got %+v", out)
	}
	if out.RedirectURI() != "http://127.0.0.1:53682/callback" {
		t.Fatal(out.RedirectURI())
	}
	cleared := ClearTokens(out)
	if cleared.AccessToken != "" || cleared.RefreshToken != "" || cleared.ClientID != "cid" {
		t.Fatalf("cleared = %+v", cleared)
	}
}

func TestLoadMissing(t *testing.T) {
	t.Setenv("XTUI_CONFIG_DIR", t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ClientID != "" {
		t.Fatal(cfg)
	}
}

func TestApplyEnv(t *testing.T) {
	t.Setenv("XTUI_CLIENT_ID", "from-env")
	t.Setenv("XTUI_ACCESS_TOKEN", "tok")
	cfg := ApplyEnv(Config{ClientID: "file"})
	if cfg.ClientID != "from-env" || cfg.AccessToken != "tok" {
		t.Fatal(cfg)
	}
}
