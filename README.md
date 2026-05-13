<p align="center">
  <img src="docs/screenshots/ScreenShot_2026-05-12_140137_931.png" alt="viewllm" width="720">
</p>

<h1 align="center">viewllm</h1>

<p align="center">
  <strong>AI agents write HTML now. This is how you view and share it.</strong>
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
  <strong><a href="https://demo.viewllm.dev">See it live →</a></strong>
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
> HTML reports are amazing. Viewing and sharing them shouldn't be this hard.

Claude Code, Codex, and Cursor are generating rich HTML reports — data visualizations, research analyses, interactive charts. HTML is replacing Markdown as the default output for AI coding agents.

But there's no good way to actually look at these files:

- **VS Code Live Preview** breaks over SSH and WSL
- **GitHub** won't render HTML — only Markdown
- **`python -m http.server`** gives you a raw directory listing
- **Sharing** means downloading files and emailing them around

Code has VS Code. Markdown has GitHub. PDF has Preview. Jupyter has nbviewer. HTML reports from AI agents had nothing — until now.

## What viewllm does
> One command. A beautiful viewer for everything your AI creates.

Run it in any project folder. You instantly get:

- **A real-time view of your AI's work** — new reports appear with blue dots as your agent produces them. You're reading one report while the next is being written.
- **Instant sharing** — send anyone a link that opens a specific report. Add `-tunnel` and it works over the internet, no port forwarding needed.
- **Collaboration built in** — everyone who opens the link gets their own unread tracking. Your team stays in sync without any setup.
- **Search and browse** — file tree, search bar with live preview, thumbnails, text snippets. Find any report in seconds.
- **HTML and Markdown** — renders both. Markdown gets GitHub-style formatting with tables, code blocks, and blockquotes.
- **Works where nothing else does** — SSH, WSL, remote servers, headless environments. If you have a terminal, you have viewllm.
- **Themes** — Light, Dark, and Solarized. Per-device settings for file types, previews, recent count, and folder excludes.

<details>
<summary>Mobile view</summary>
<p align="center">
  <img src="docs/screenshots/Image_20260511115901_15_618.jpg" alt="viewllm mobile" width="280">
</p>
</details>

## Ultralight and fast
> 9MB binary. 7MB RAM. Starts in 100ms. Leaves no trace.

| | |
|---|---|
| **Binary size** | 9MB |
| **Memory usage** | ~7MB |
| **Startup to first page** | ~100ms |
| **API response** | <5ms |
| **Dependencies** | 0 |
| **Trace left on your system** | None |

Not another Electron app — it's a single Go binary closer to a Unix utility. Read-only. No database, no config files, no background processes. Stop it and it's gone.

## Try this right now
> 30 seconds to see why people don't go back to Markdown.

If you're using an AI coding agent, paste this prompt:

> *"Write me an HTML report analyzing the architecture of this codebase. Include diagrams, dependency graphs, and your recommendations."*

Then run:

```bash
npx viewllm@latest
```

That's the workflow. Your AI builds rich, visual reports. viewllm lets you see and share them instantly. Once you try it, Markdown feels like reading a spreadsheet printout.

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
> Your reports are only useful if others can see them.

viewllm shows you a link on startup that works for you and anyone on your network:

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

## Works with every AI coding tool

viewllm doesn't care who made the file. It works with **Claude Code**, **Codex**, **Cursor**, **Jupyter**, and anything else that outputs `.html` or `.md` files.

## Usage

```
viewllm [directory] [-p port] [-exclude dir]... [-tunnel]
```

Serves the current directory by default. Finds an open port automatically.

| Flag | Default | Description |
|------|---------|-------------|
| `-p` | `8090` | Port to serve on |
| `-exclude` | — | Additional directories to ignore (repeatable) |
| `-tunnel` | off | Create a public URL via Cloudflare Tunnel |

<details>
<summary>Settings</summary>

Click the gear icon to access per-device settings:

- **Show** — toggle HTML and Markdown files on/off
- **Theme** — Light, Dark, Solarized
- **Text preview** — show/hide text snippets
- **Thumbnail preview** — show/hide live mini-renders
- **Recent files count** — 0 (off), 3, 5, 10, 15, or 20
- **Ignored folders** — add/remove custom exclude patterns

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

Single Go binary with the entire frontend embedded via `go:embed`. Scans for `.html` files, serves a web UI, polls for changes every 2 seconds.

**Stack:** Go stdlib (zero deps) · Vanilla HTML/CSS/JS · `go:embed` · Polling-based file discovery

</details>

## Contributing

The codebase is intentionally simple — two files:

```
main.go              → Server, API, file scanning (~600 lines)
frontend/index.html  → Entire UI, embedded into the binary (~35KB)
```

## License

MIT

