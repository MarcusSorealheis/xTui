# xTui

xTui is a terminal client for the signed-in X home timeline. It talks to the official X API v2 with OAuth 2.0 and PKCE. It does not ask for your X password.

The source is [Apache 2.0](LICENSE) and lives at [github.com/MarcusSorealheis/xTui](https://github.com/MarcusSorealheis/xTui).

The screen is three panes when the terminal is wide enough: feeds on the left, the timeline in the middle, and the open post with its action menu on the right. A two-line key menu stays pinned to the bottom. Colors follow the terminal's light or dark appearance.

A terminal at least 100 columns wide shows all three panes. From 92 columns the detail pane remains and the feed list drops. Narrower windows keep the timeline and the key menu. Below 40×10 the app asks you to resize.

## Build and run

Go 1.24 or newer.

```bash
go install github.com/MarcusSorealheis/xTui@latest
```

Or build from this directory:

```bash
go build -o xtui .
./xtui --demo     # local feed, no network
./xtui            # your account
./xtui --auth     # forget the session and sign in again
./xtui --logout   # forget the session and exit
```

`--demo` never reads or writes your account. `d` on the sign-in screen does the same thing.

Rebuilding replaces the `xtui` program only. The client id and tokens stay in the config file, so a source change does not make you sign in again. You sign in again if you delete that file, run `./xtui --logout`, or point `XTUI_CONFIG_DIR` at a different directory. `--auth` clears the tokens and keeps the client id, then opens sign-in.

Quit the running copy before you start the new binary. `q` or `ctrl+c` leaves the app.

## Sign in

Create an app in the [X developer console](https://developer.x.com).

1. Turn on user authentication with OAuth 2.0. A native app is a public client and needs only a client id. A confidential web app also has a client secret.
2. Register this callback exactly: `http://127.0.0.1:53682/callback`
3. Allow the scopes listed below. The consent screen shows the same set.
4. Copy the client id from Keys and tokens.

Then:

```bash
./xtui
```

The first launch opens on the client-id field. Paste the id and press enter. The browser opens. Approve xTui. The loopback page says you can close the tab, and the timeline loads.

The client id is remembered. The next launch opens the browser on its own when that id is saved and the session is missing. A saved session goes straight to the timeline.

| Sign-in key | Action |
| --- | --- |
| `enter` | Save the client id and open the browser, or open the browser when the id is already saved |
| `ctrl+s` | Set a client secret, then return to the client id. Leave it empty for a public app |
| `esc` | Leave the field, or cancel the browser wait |
| `q` | Cancel the browser wait, or quit from the sign-in screen |
| `o` | Open the sign-in page again while waiting |
| `i` | Edit the saved client id |
| `s` | Edit the secret from the sign-in screen |
| `d` | Open the demo |
| `ctrl+c` | Quit |

The wait screen counts seconds so you can tell it is still waiting. The sign-in link is shown there if the browser does not come forward.

`redirect_port` in the config file changes the loopback port. The callback you register must match `http://127.0.0.1:<port>/callback`. The default port is 53682.

### Scopes

`tweet.read`, `tweet.write`, `users.read`, `like.read`, `like.write`, `bookmark.read`, `bookmark.write`, `follows.read`, `follows.write`, `mute.read`, `mute.write`, `block.read`, `block.write`, `list.read`, `offline.access`, `tweet.moderate.write`.

`offline.access` is what lets a later launch refresh the session without the browser.

### Where credentials live

The config file is `xtui/config.json` inside the OS config directory, mode `0600`:

| System | Path |
| --- | --- |
| macOS | `~/Library/Application Support/xtui/config.json` |
| Linux | `${XDG_CONFIG_HOME:-~/.config}/xtui/config.json` |

`XTUI_CONFIG_DIR` replaces that directory. These variables override the file when they are set: `XTUI_CLIENT_ID`, `XTUI_CLIENT_SECRET`, `XTUI_ACCESS_TOKEN`, `XTUI_REFRESH_TOKEN`.

The file holds the client id, the optional secret, the access token, the refresh token, and the expiry. It stays on your machine, outside this repository, with mode `0600`. The repository ignores every file except Go source, `go.mod`, `go.sum`, `README.md`, and the license. JSON, YAML, env files, private keys, and the usual credential names are ignored again at the bottom of `.gitignore`, so they stay out even if the allowlist grows. Sign-out clears the tokens and leaves the app credentials.

## The screen

The header shows the xTui badge, the feed name, your handle, a demo marker when you are offline, the selection index, and `rl` plus the last `X-Rate-Limit-Remaining` value.

The left pane is the feed list:

| Key | Feed |
| --- | --- |
| `1` | Home, reverse chronological |
| `2` | Mentions |
| `3` | Bookmarks |
| `4` | Likes |
| `5` | Your posts |
| `6` | Lists you own and lists you follow |

Enter on a list opens its posts. `u` opens the selected author's posts. `/` searches recent posts. `esc` steps back through search, an author, a list, and a thread. The feed keys clear that stack and load the numbered feed.

The timeline row shows the author, the relative time, one or two lines of text, and counts for likes, reposts, and replies. A repost row shows the original post and who reposted it. A reply or quote is marked on the metrics line. Moving to the last loaded row fetches the next page.

The right pane is the open post: text, quote, media description, counts, permalink, and the action menu. `tab` and `shift+tab` move between panes. With the detail pane focused, `j` and `k` scroll that pane. The detail pane is hidden on a narrow terminal. Enter still opens the thread in the timeline.

The status line under the panes is red when the last action failed.

## Keys

Press `?` for the same list inside the app. `esc`, `q`, or `?` closes help.

### Move

| Key | Action |
| --- | --- |
| `j`, `down` | Next row |
| `k`, `up` | Previous row |
| `g` | First row |
| `G` | Last loaded row, and the next page when one exists |
| `ctrl+d`, `pgdown` | Page down |
| `ctrl+u`, `pgup` | Page up |
| `tab` | Next pane |
| `shift+tab` | Previous pane |
| `enter` | Open the thread, or open the selected list |
| `esc` | Back |
| `R` | Refresh the current feed |

### Act on a post

| Key | Action | Confirm |
| --- | --- | --- |
| `l` | Like or unlike | No |
| `b` | Bookmark or remove it | No |
| `t` | Repost or undo | `y` |
| `T` | Quote | The composer, then `ctrl+p` |
| `r` | Reply | The composer, then `ctrl+p` |
| `n` | New post | The composer, then `ctrl+p` |
| `f` | Follow or unfollow the author | `y` |
| `m` | Mute or unmute the author | `y` |
| `B` | Block or unblock the author | `y` |
| `h` | Hide or unhide a reply to you | `y` |
| `d` | Delete your own post | `y` |
| `c` | Copy the permalink | No |
| `o` | Open the permalink in a browser | No |
| `u` | That author's posts | No |

Like and bookmark change on screen immediately and revert if the API refuses them. Repost, follow, mute, block, hide, and delete wait for `y`. `n`, `esc`, or `q` cancels. Enter does not confirm.

In the composer, `enter` inserts a newline, `ctrl+p` sends, and `esc` cancels. An empty draft is not sent.

`d` refuses a post you did not write, before any request. Hide applies to a reply to one of your posts.

### Search and the app

| Key | Action |
| --- | --- |
| `/` | Search recent posts. `enter` runs it, `esc` cancels |
| `?` | Help |
| `ctrl+x` | Sign out and return to the sign-in screen |
| `q`, `ctrl+c` | Quit |

Search accepts a recent-search query, such as a phrase or `from:handle`. A thread is a `conversation_id` search plus the root post. Recent search covers about seven days, so older replies are left out. The root post is still shown when the search itself fails.

## What the API allows

Home is `GET /2/users/:id/timelines/reverse_chronological`. Mentions, bookmarks, likes, a user's posts, and list posts are the matching v2 timelines. Each page asks for 20 posts. Timelines page with `pagination_token`. Recent search pages with `next_token`.

New posts, replies, and quotes use `POST /2/tweets`. Since February 2026, self-serve API tiers accept a reply or a quote only when you wrote the original post, or when that author has @mentioned you or quoted you. A reply to anyone else returns: "You can only reply to or quote posts where you are mentioned or are the author." New posts are unaffected. Likes, bookmarks, and reposts use their own endpoints and are outside that rule. Enterprise and Public Utility apps are exempt. The X website and the official X apps are also unaffected.

The access token is refreshed when it is within 45 seconds of expiry, and once after a 401. The new token is written back to the config file.

## Demo

`./xtui --demo` loads a fixed local account, @ada, with a home timeline, mentions, and two lists. Likes, bookmarks, reposts, posts, and deletes update that memory only. Nothing is sent to X. The home page is four posts, and `G` or moving to the end loads the rest.

## Develop

```
main.go                 flags, config, and the Bubble Tea program
internal/config         config file and environment overrides
internal/xapi           OAuth, the v2 client, response parsing, and the demo
internal/ui             keys, layout, and the screen
```

The module path is `github.com/MarcusSorealheis/xTui`. UI code uses Bubble Tea, Bubbles, and Lip Gloss. `internal/ui/style.go` holds the palette.

```bash
go test ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development setup, the ignore rules, and what a change should include.

Config tests use a temporary directory. API tests use `httptest` and the in-memory demo. UI tests drive keystrokes and check the rendered frame, including line width at 80, 100, 120, and 160 columns. No test calls X.

## License

Copyright 2026 Marcus Eagan.

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE). You can use, modify, and distribute this software under those terms.
