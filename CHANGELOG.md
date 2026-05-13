# Changelog

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
