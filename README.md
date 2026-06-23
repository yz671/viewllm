<p align="center">
  <img src="docs/screenshots/ScreenShot_2026-05-12_140137_931.png" alt="viewllm" width="720">
</p>

<h1 align="center">viewllm</h1>

<p align="center">
  <strong>View and share HTML/Markdown reports from AI coding agents.</strong>
</p>

<p align="center">
  Run this in your terminal — no install needed:
</p>

<p align="center">

```bash
npx viewllm@latest
```

</p>

<p align="center">
  <strong><a href="https://demo.viewllm.dev">Live demo →</a></strong>
</p>

<p align="center">
  <sub><a href="https://github.com/yz671/viewllm/releases">Download binary</a> · <a href="https://www.npmjs.com/package/viewllm">npm</a></sub>
</p>

---

> "HTML is the new markdown. I've stopped writing markdown files for almost everything and switched to using Claude Code to generate HTML for me."
>
> — [**Thariq Shihipar**](https://x.com/trq212/status/2052811606032269638), Engineering Lead, Claude Code at Anthropic · 5M+ views

---

## The problem

Claude Code, Codex, and Cursor generate rich HTML reports — data visualizations, research analyses, interactive charts. But there's no good way to view or share them:

- **VS Code Live Preview** breaks over SSH and WSL
- **GitHub** won't render HTML — only Markdown
- **`python -m http.server`** gives you a raw directory listing
- **Sharing** means downloading files and emailing them around

## What viewllm does

Run it in any project folder. You get:

- **Real-time file watching** — new and modified files appear with unread indicators as your agent produces them
- **Sharing** — shareable links to specific reports. Add `-tunnel` for a public URL, no port forwarding needed
- **Per-device unread tracking** — each person who opens the link gets their own read/unread state
- **Search** — file tree, search bar, hover preview popups on all file items, thumbnails, text snippets
- **HTML and Markdown** — renders both. Markdown gets GitHub-style formatting
- **Browse GitHub repos** — view reports straight from a private GitHub repository, even after the machine that generated them is gone
- **SSH, WSL, remote servers** — works anywhere you have a terminal
- **Themes** — Light, Dark, Solarized

<details>
<summary>Mobile view</summary>
<p align="center">
  <img src="docs/screenshots/Image_20260511115901_15_618.jpg" alt="viewllm mobile" width="280">
</p>
</details>

## Lightweight

| | |
|---|---|
| **Binary** | 9MB |
| **Memory** | ~7MB |
| **Startup** | ~100ms |
| **API response** | <5ms |
| **Dependencies** | 0 |

Single Go binary. No database, no config files, no background processes. Read-only. Stop it and it's gone.

## Quick start

Ask your AI coding agent:

> *"Write me an HTML report analyzing the architecture of this codebase. Include diagrams, dependency graphs, and your recommendations."*

Then:

```bash
npx viewllm@latest
```

## Install

**npx** (zero install, always latest):
```bash
npx viewllm@latest
```

**Binary** — grab from [GitHub Releases](https://github.com/yz671/viewllm/releases):
```bash
curl -fsSL https://github.com/yz671/viewllm/releases/latest/download/viewllm-linux-amd64 -o viewllm
chmod +x viewllm
./viewllm
```

<details>
<summary>Build from source</summary>

```bash
git clone https://github.com/yz671/viewllm.git && cd viewllm
go build -o viewllm . && ./viewllm
```

</details>

## Sharing

viewllm shows a shareable link on startup:

```
viewllm serving ./reports

  Open: http://192.168.1.42:8090
  To share over the internet, use -tunnel
```

For sharing beyond your network — no accounts, no port forwarding:

```bash
npx viewllm@latest -tunnel
```

```
viewllm serving ./reports — starting tunnel (powered by Cloudflare)...

  Share this link: https://random-words.trycloudflare.com

  Anyone with this link can view your reports.
  This link expires when you stop viewllm — a new one is created each time.
```

## Browse a GitHub repository

viewllm can read reports directly from a GitHub repository — useful when the machine that generated them is no longer running, such as a CI job, a short-lived cloud instance, or a teammate's laptop. Commit your `.html`/`.md` reports to a repo, then open them from anywhere.

1. Start viewllm with no directory: `npx viewllm@latest`
2. Click the gear icon, then under **Source** choose **Browse GitHub repository…**
3. Enter the repo as `owner/repo` (or paste its GitHub URL), add a token for private repos, and click **Connect**

The file tree, recent files, and search all work exactly as they do in local mode. The full file list is fetched in a single GitHub API call; individual reports are downloaded on demand and rendered in your browser, with nothing written to disk. The repo and token are saved in your browser's localStorage, so viewllm reconnects automatically next time.

### Generating a token

Public repositories work without a token, subject to GitHub's unauthenticated rate limit of 60 requests per hour.

For **private** repositories you need a personal access token. viewllm only ever reads, so grant the most minimal read-only access:

- **Recommended — fine-grained token.** [Create one here](https://github.com/settings/personal-access-tokens/new). Under **Repository access**, choose *Only select repositories* and pick your reports repo. Under **Permissions → Repository permissions**, set **Contents: Read-only** — that is the only permission needed (Metadata: Read-only is added automatically). This token can read just that one repo and cannot write anything.
- **Classic token.** Simpler but much broader: the `repo` scope grants full read **and write** access to all of your private repositories. Use it only if a fine-grained token isn't an option.

The token is sent only to GitHub's API, straight from your browser. An authenticated token raises the rate limit to 5,000 requests per hour, which is far more than browsing reports needs.

## Works with any tool

Works with **Claude Code**, **Codex**, **Cursor**, **Jupyter**, and anything else that outputs `.html` or `.md` files.

## Usage

```
viewllm [directory] [-p port] [-exclude dir]... [-exclude-file name]... [-tunnel]
```

Serves the current directory by default. Finds an open port automatically.

| Flag | Default | Description |
|------|---------|-------------|
| `-p` | `8090` | Port to serve on |
| `-exclude` | — | Additional directories to ignore (repeatable) |
| `-exclude-file` | — | Additional files to ignore by name (repeatable) |
| `-tunnel` | off | Create a public URL via Cloudflare Tunnel |

<details>
<summary>Settings</summary>

Click the gear icon to access per-device settings:

- **Show** — toggle HTML and Markdown files on/off
- **Theme** — Light, Dark, Solarized
- **Text preview** — show/hide text snippets
- **Thumbnail preview** — show/hide live mini-renders
- **Recent files count** — 0 (off), 3, 5, 10, 15, or 20
- **Ignored folders & files** — add/remove custom exclude patterns for both folders and files
- **Source** — browse a GitHub repository instead of local files

All settings stored in localStorage — each device has its own preferences.

</details>

<details>
<summary>API</summary>

```
GET /              → Web UI
GET /api/recent    → Recently modified files (with previews)
GET /api/tree      → Full directory tree as nested JSON
GET /api/excludes  → Current exclude patterns
GET /files/{path}  → Serves HTML directly, renders Markdown as styled HTML
```

</details>

<details>
<summary>Technical details</summary>

Single Go binary with the entire frontend embedded via `go:embed`. Scans for `.html` and `.md` files, serves a web UI, polls for changes every 2 seconds. Markdown rendered client-side with [marked.js](https://github.com/markedjs/marked).

**Stack:** Go stdlib (zero deps) · Vanilla HTML/CSS/JS · `go:embed` · Polling-based file discovery

</details>

## Contributing

The codebase is intentionally simple — two files:

```
main.go              → Server, API, file scanning (~865 lines)
frontend/index.html  → Entire UI, embedded into the binary (~70KB)
```

## License

MIT
