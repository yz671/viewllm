# Changelog

## v0.6.3 (2026-06-24)

- **Previews for GitHub reports** — recent reports from a GitHub repo now show thumbnail renders (inline in the list and in the hover popup) plus text snippets, matching local mode. Content is fetched on demand — inline thumbnails lazy-load only as they scroll into view — and cached by commit SHA, so it's one fetch per report, shared with opening it. Controlled by the existing Text/Thumbnail preview toggles in settings
- **Taller hover preview** — the hover preview popup is now A4-proportioned, showing a full page of the report instead of just the top

## v0.6.2 (2026-06-24)

- **Report-to-report links in GitHub repos** — clicking a link from one report to another now loads the linked report instead of a blank page. GitHub reports render from blob URLs, which have no path to resolve relative links against, so viewllm now intercepts those links and resolves them against the report's path in the repo
- **Header follows in-report navigation** — when you click a link between reports, the file path shown at the top, the sidebar highlight, and the URL all update to match the report you landed on (previously only the report body changed)

## v0.6.1 (2026-06-24)

- **Reliable "most recent" for GitHub repos** — the Recent list now orders reports by the date of the commit that last changed them, found by scanning recent commits in a single pass. This replaces a per-file date lookup that could fail under rate limits and silently drop the newest report out of Recent
- **Markdown hidden by default** — `.md` files are no longer shown until you turn them on in settings (Show → Markdown); the default view is HTML-only
- **Recent backfill** — when one file type is hidden, the Recent list fills to the configured count from the other type instead of showing fewer items
- **Faster GitHub report reopening** — a report's rendered content is cached after first open (keyed by its commit SHA) and reused instead of being re-downloaded

## v0.6.0 (2026-06-23)

- **Browse GitHub repositories** — view HTML/Markdown reports straight from a GitHub repo, even after the machine that generated them is gone (a CI job, a spot instance, a teammate's laptop). Open the **Source** section in settings, enter `owner/repo`, and connect
- **Private repo support** — authenticate with a GitHub personal access token; the in-app help links to GitHub's fine-grained token page and recommends granting only **Contents: Read-only** access. Public repos work with no token
- **Single-call file listing** — the full repo tree is fetched in one GitHub Trees API request; file contents load on demand as blobs and render in the browser, with nothing written to disk
- **Recent-commit sorting** — recent files are ordered by each file's last commit date, fetched in parallel after the tree loads
- **Persistent connection** — the connected repo and token are saved in localStorage, so viewllm reconnects automatically on the next visit; a source bar in the sidebar shows the active repo with refresh and disconnect controls

## v0.5.7 (2026-05-15)

- **File-level excludes** — ignore specific files (not just folders) by name. Default file excludes: `CLAUDE.md`, `AGENTS.md`, `.cursorrules`, `.cursorignore`, `.aiderignore`, `.gitignore`, `.dockerignore`, `Thumbs.db`, `.DS_Store`
- **`-exclude-file` CLI flag** — add custom file excludes from the command line (repeatable)
- **Folder/file type selector in settings** — dropdown to choose between folder and file when adding custom excludes; icons distinguish the two types
- **File path in header bar** — current file's directory and name displayed centered in the top bar, aligned over the content area
- **Hover preview on all file items** — the large preview popup (previously search-only) now appears when hovering recent files and folder tree items

## v0.5.4 (2026-05-13)

- **Dark theme fix** — moved `data-theme` to `<html>` so all CSS variables cascade correctly; boosted text and border contrast
- **Solarized theme fix** — darkened text and muted colors for better readability
- **Author credit** — settings footer now shows "by yz671"

## v0.5.3 (2026-05-13)

- **Client-side custom excludes** — custom folder excludes now stored in localStorage per-user, sent with each request; server stays stateless (no trace on disk)
- **Expanded default excludes** — added `.claude`, `.codex`, `.aider`, `.cursor`, `.vscode-server`, `.idea`, `target`, `vendor`, `coverage`
- **Exclude help tooltip** — `?` icon next to "Ignored Folders" explains exact-match behavior with examples

## v0.5.2 (2026-05-13)

- **Public IP detection** — on cloud VMs (AWS, GCP, Azure), detects and displays the public IP as a shareable link instead of suggesting `-tunnel`
- **Zero delay on non-cloud** — skips detection on WSL; instant failure on machines without cloud metadata

## v0.5.0 (2026-05-13)

- **Markdown rendering** — `.md` files now render with GitHub-style CSS (tables, code blocks, blockquotes, heading borders)
- **File type toggles** — show/hide HTML and Markdown independently in settings
- **Image serving** — markdown image references (png, jpg, gif, svg, webp) render correctly
- **Thumbnail previews for markdown** — compact layout optimized for sidebar thumbnails

## v0.4.2 (2026-05-12)

- **Improved startup output** — shows shareable network link instead of localhost
- **Environment hints** — detects WSL, SSH, and NAT; suggests `-tunnel` when relevant
- **Tunnel UX** — friendly messages explaining Cloudflare tunnel URLs, link expiry, and attribution

## v0.4.1 (2026-05-12)

- **Default to current directory** — no directory argument required, serves `.` by default
- **Auto-increment port** — if default port 8090 is taken, automatically finds the next available port
- **Search hover preview** — hovering search results shows a preview panel with thumbnail and file details

## v0.4.0 (2026-05-12)

- **Search** — search bar with SVG magnifying glass icon, filters files by path
- **Two-line search results** — filename (bold) on first line, directory path (muted) on second
- **Folder state persistence** — expanded/collapsed folders preserved across re-renders
- **Collapsible recent section** — toggle the recent files list open/closed

## v0.3.1 (2026-05-11)

- **Security fixes** — path traversal prevention with `filepath.Abs` + prefix check, XSS prevention with `esc()` function
- **Separate preview cache mutex** — reduced lock contention between file scanning and preview generation
- **Stale cache cleanup** — preview cache entries removed when files are deleted

## v0.3.0 (2026-05-11)

- **Cloudflare Tunnel** — `-tunnel` flag creates a public URL for sharing behind NAT/firewall/WSL
- **Auto-detection** — finds `cloudflared` binary or falls back to `npx cloudflared`

## v0.2.2 (2026-05-11)

- **Update check** — non-blocking check against npm registry on startup, prints "Update available" if newer version exists

## v0.2.1 (2026-05-11)

- **Version display** — version number and GitHub link in settings footer
- **Iframe cache busting** — appends `?v={mtime}` to iframe src to prevent stale content

## v0.2.0 (2026-05-11)

- **Welcome page** — feature guide with 4-card grid shown when no file is selected
- **Mobile improvements** — fixed sidebar edge gap, responsive layout refinements
- **Unread badge fix** — markAsRead and markAllAsRead now properly trigger re-render
- **Screenshots** — added desktop and mobile screenshots to README

## v0.1.0 (2026-05-11)

- **Initial release** — single Go binary HTML report viewer
- **Split-pane UI** — sidebar with file tree + full-width iframe viewer
- **Recent files** — configurable top-N most recently modified files
- **Unread tracking** — blue dots on new/modified files, per-device via localStorage
- **Text previews** — extracted text snippets in sidebar
- **Thumbnail previews** — scaled iframe renders of recent reports
- **Themes** — Light, Dark (Tokyo Night), Solarized
- **Settings panel** — theme, preview toggles, recent count, exclude patterns
- **Directory excludes** — auto-skips venv, node_modules, .git, etc.
- **Pull-to-refresh** — mobile gesture to reload current report
- **npm distribution** — `npx viewllm` with postinstall binary download
- **GitHub Actions** — cross-platform builds on tag push
