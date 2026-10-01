# Contributing to xTui

Thanks for looking. xTui is licensed under the Apache License 2.0. A contribution sent for inclusion is covered by that license. See [LICENSE](LICENSE).

## Setup

Go 1.24 or newer.

```bash
go test ./...
go build -o xtui .
./xtui --demo
```

`go test ./...` needs no network and no X account. Config tests use a temporary directory. API tests use `httptest` and the in-memory demo. UI tests send keystrokes and check the rendered frame.

`--demo` is the way to try the screen. It does not read or write `~/Library/Application Support/xtui/config.json`.

## What goes where

| Path | Responsibility |
| --- | --- |
| `main.go` | Flags, loading config, starting Bubble Tea |
| `internal/config` | The config file and `XTUI_*` environment overrides |
| `internal/xapi` | OAuth 2.0 PKCE, the X API v2 client, parsing, and the demo |
| `internal/ui` | Keys, layout, and drawing. Colors are in `style.go` |

The module path is `github.com/MarcusSorealheis/xTui`. Run `gofmt` on Go files you change.

## Secrets

Do not put a client id, client secret, access token, refresh token, or a copy of `config.json` in a commit, a test, a fixture, or an issue.

The repository ignores every file except Go source, `go.mod`, `go.sum`, `README.md`, `CONTRIBUTING.md`, `LICENSE`, and the demo files `xTui-demo.gif` and `xTui-demo.mov`. JSON, YAML, env files, private keys, and credential names are ignored again at the bottom of `.gitignore`. To track a new kind of file, add a `!` rule in the allowlist at the top of `.gitignore`, above that credential block. Check it with:

```bash
git check-ignore -v --no-index path/to/the-file
git status --short --untracked-files=all
```

A tracked source file can still contain a pasted token. Read the diff before you commit.

## Behavior to preserve

- The client uses the official X API v2 only. OAuth uses PKCE. The app does not ask for an X password.
- The demo service must stay offline. A demo action updates memory and does not call the network.
- Like and bookmark apply immediately and revert when the API returns an error. Repost, follow, mute, block, hide, and delete wait for `y`.
- Delete refuses a post the signed-in user did not write, before any request.
- Since February 2026, self-serve X API tiers accept a reply or a quote only when you wrote the original post, or when that author has @mentioned you or quoted you. Surface that API error. Do not route around it with an unofficial client.
- Home, mentions, bookmarks, likes, posts, and lists page with `pagination_token`. Recent search pages with `next_token`. A thread is a recent `conversation_id` search plus the root post, so replies older than that window can be missing.
- Tokens are refreshed within 45 seconds of expiry and once after a 401, then written back to the local config file.

## A UI change

Add or update a test in `internal/ui` that drives the key the way a person would. `go test ./internal/ui` checks that the frame has the expected line count and that each line is the terminal width at 80, 100, 120, and 160 columns. Run `./xtui --demo` and use the changed key before you send the change.

`internal/ui/keys.go` is the list behind `?`. A new action needs a binding there and a handler in `update.go`.

## Pull requests

Use a focused change. In the description, say what changed, why, and which tests you ran. If you could not run a test, say so.

Report a security problem in private mail to the address on the GitHub profile rather than in a public issue. Do not include a live token in the report.
