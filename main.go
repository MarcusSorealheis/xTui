package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/MarcusSorealheis/xTui/internal/config"
	"github.com/MarcusSorealheis/xTui/internal/ui"
	"github.com/MarcusSorealheis/xTui/internal/xapi"
)

func main() {
	demo := flag.Bool("demo", false, "open a local demo feed with no network")
	auth := flag.Bool("auth", false, "forget the saved session and sign in again")
	logout := flag.Bool("logout", false, "forget the saved session and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: xtui [--demo | --auth | --logout]\n\n")
		fmt.Fprintf(os.Stderr, "xTui is a terminal client for the signed-in X home timeline.\n")
		fmt.Fprintf(os.Stderr, "Press ? inside the app for every key.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	cfg = config.ApplyEnv(cfg)
	store := config.NewStore(cfg)

	if *logout {
		if err := store.Update(func(c *config.Config) { *c = config.ClearTokens(*c) }); err != nil {
			fail(err)
		}
		fmt.Println("signed out")
		return
	}
	if *auth {
		if err := store.Update(func(c *config.Config) { *c = config.ClearTokens(*c) }); err != nil {
			fail(err)
		}
		cfg = store.Get()
	}

	var svc xapi.Service
	var me xapi.User
	var startErr string
	if *demo {
		d := xapi.NewDemo()
		svc = d
		me, _ = d.Me(context.Background())
	} else if cfg.AccessToken != "" {
		svc = newClient(store)
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		me, err = svc.Me(ctx)
		cancel()
		if err != nil {
			svc = nil
			startErr = err.Error()
		}
	}

	m := ui.New(ui.Options{Service: svc, Me: me, Store: store, Demo: *demo, Err: startErr})
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fail(err)
	}
}

func newClient(store *config.Store) *xapi.Client {
	cfg := store.Get()
	return xapi.New(xapi.Options{
		AccessToken:  cfg.AccessToken,
		RefreshToken: cfg.RefreshToken,
		Expiry:       cfg.Expiry,
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Persist: func(tok xapi.Token) {
			_ = store.Update(func(c *config.Config) {
				c.AccessToken = tok.AccessToken
				if tok.RefreshToken != "" {
					c.RefreshToken = tok.RefreshToken
				}
				c.Expiry = tok.Expiry
			})
		},
	})
}

func fail(err error) {
	if err == nil || errors.Is(err, context.Canceled) {
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "xtui: %s\n", err)
	os.Exit(1)
}
