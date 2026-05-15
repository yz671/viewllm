package main

import (
	"bufio"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed frontend/index.html
var frontendFS embed.FS

//go:embed frontend/marked.min.js
var markedJS []byte

type FileInfo struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Dir     string `json:"dir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mtime"`
	Preview string `json:"preview,omitempty"`
}

func truncateAtWord(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	text = text[:maxLen]
	if j := strings.LastIndex(text, " "); j > maxLen/2 {
		text = text[:j]
	}
	return text + "..."
}

func extractMarkdownPreview(path string, maxLen int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var preview strings.Builder
	for i := 0; i < 30 && scanner.Scan(); i++ {
		trimmed := strings.TrimSpace(scanner.Text())
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "===") || strings.HasPrefix(trimmed, "<") || strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "![") {
			continue
		}
		if preview.Len() > 0 {
			preview.WriteByte(' ')
		}
		preview.WriteString(trimmed)
		if preview.Len() >= maxLen {
			break
		}
	}
	return truncateAtWord(strings.TrimSpace(preview.String()), maxLen)
}

func extractPreview(path string, maxLen int) string {
	if strings.HasSuffix(strings.ToLower(path), ".md") {
		return extractMarkdownPreview(path, maxLen)
	}

	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil || info.Size() == 0 {
		return ""
	}

	// Read up to 512KB to find <body>, then extract 16KB from there
	scanSize := int64(524288)
	if info.Size() < scanSize {
		scanSize = info.Size()
	}
	header := make([]byte, scanSize)
	n, _ := f.Read(header)
	if n == 0 {
		return ""
	}

	raw := string(header[:n])
	if idx := strings.Index(strings.ToLower(raw), "<body"); idx >= 0 {
		raw = raw[idx:]
	}
	if len(raw) > 16384 {
		raw = raw[:16384]
	}

	var b strings.Builder
	inTag := false
	inSkip := false
	i := 0
	for i < len(raw) {
		if !inTag && raw[i] == '<' {
			rest := strings.ToLower(raw[i:])
			if strings.HasPrefix(rest, "<script") || strings.HasPrefix(rest, "<style") || strings.HasPrefix(rest, "<head") {
				var closeTag string
				if strings.HasPrefix(rest, "<script") {
					closeTag = "</script>"
				} else if strings.HasPrefix(rest, "<style") {
					closeTag = "</style>"
				} else {
					closeTag = "</head>"
				}
				end := strings.Index(strings.ToLower(raw[i:]), closeTag)
				if end >= 0 {
					i += end + len(closeTag)
					inSkip = false
					continue
				}
				inSkip = true
			}
			inTag = true
			i++
			continue
		}
		if inTag {
			if raw[i] == '>' {
				inTag = false
			}
			i++
			continue
		}
		if inSkip {
			i++
			continue
		}
		if raw[i] == '&' {
			semi := strings.IndexByte(raw[i:], ';')
			if semi > 0 && semi < 10 {
				i += semi + 1
				b.WriteByte(' ')
				continue
			}
		}
		ch := raw[i]
		if ch == '\n' || ch == '\r' || ch == '\t' {
			ch = ' '
		}
		if ch == ' ' && b.Len() > 0 {
			s := b.String()
			if s[len(s)-1] == ' ' {
				i++
				continue
			}
		}
		b.WriteByte(ch)
		i++
	}

	return truncateAtWord(strings.TrimSpace(b.String()), maxLen)
}

type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path,omitempty"`
	Children []*TreeNode `json:"children,omitempty"`
	IsFile   bool        `json:"isFile"`
	ModTime  int64       `json:"mtime,omitempty"`
}

var defaultExcludes = []string{
	"venv", ".venv", "node_modules", ".git", "__pycache__",
	".tox", ".mypy_cache", ".pytest_cache", ".eggs", "dist",
	"build", ".next", ".nuxt", ".cache",
	".claude", ".codex", ".aider", ".cursor",
	".vscode-server", ".idea",
	"target", "vendor", "coverage",
}

var defaultFileExcludes = []string{
	"CLAUDE.md", "AGENTS.md", ".cursorrules", ".cursorignore",
	".aiderignore", ".gitignore", ".dockerignore",
	"Thumbs.db", ".DS_Store",
}

type previewCache struct {
	mtime   int64
	preview string
}

type Server struct {
	rootDir         string
	mu              sync.RWMutex
	files           []FileInfo
	previewMu       sync.Mutex
	previews        map[string]previewCache
	cliExcludes     []string
	cliFileExcludes []string
}

func NewServer(rootDir string, cliExcludes, cliFileExcludes []string) *Server {
	s := &Server{rootDir: rootDir, cliExcludes: cliExcludes, cliFileExcludes: cliFileExcludes, previews: make(map[string]previewCache)}
	s.scan()
	go s.pollLoop()
	return s
}

func (s *Server) isExcluded(name string) bool {
	for _, pat := range defaultExcludes {
		if strings.EqualFold(name, pat) {
			return true
		}
	}
	for _, pat := range s.cliExcludes {
		if strings.EqualFold(name, pat) {
			return true
		}
	}
	return false
}

func (s *Server) isFileExcluded(name string) bool {
	for _, pat := range defaultFileExcludes {
		if strings.EqualFold(name, pat) {
			return true
		}
	}
	for _, pat := range s.cliFileExcludes {
		if strings.EqualFold(name, pat) {
			return true
		}
	}
	return false
}

type clientExcludes struct {
	folders []string
	files   []string
}

func parseClientExcludes(r *http.Request) clientExcludes {
	raw := r.URL.Query().Get("excludes")
	if raw == "" {
		return clientExcludes{}
	}
	var ce clientExcludes
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if strings.HasPrefix(s, "file:") {
			name := strings.TrimPrefix(s, "file:")
			if name != "" {
				ce.files = append(ce.files, name)
			}
		} else {
			ce.folders = append(ce.folders, s)
		}
	}
	return ce
}

func matchesClientExclude(filePath string, ce clientExcludes) bool {
	name := filepath.Base(filePath)
	for _, ex := range ce.files {
		if strings.EqualFold(name, ex) {
			return true
		}
	}
	dir := filepath.Dir(filePath)
	if dir != "." {
		for _, part := range strings.Split(dir, string(filepath.Separator)) {
			for _, ex := range ce.folders {
				if strings.EqualFold(part, ex) {
					return true
				}
			}
		}
	}
	return false
}

func (s *Server) scan() {
	var files []FileInfo
	filepath.WalkDir(s.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != s.rootDir && s.isExcluded(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if s.isFileExcluded(d.Name()) {
			return nil
		}
		lower := strings.ToLower(d.Name())
		if !strings.HasSuffix(lower, ".html") && !strings.HasSuffix(lower, ".md") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(s.rootDir, path)
		dir := filepath.Dir(rel)
		if dir == "." {
			dir = ""
		}
		files = append(files, FileInfo{
			Path:    rel,
			Name:    d.Name(),
			Dir:     dir,
			Size:    info.Size(),
			ModTime: info.ModTime().Unix(),
		})
		return nil
	})
	s.mu.Lock()
	s.files = files
	s.mu.Unlock()

	s.previewMu.Lock()
	active := make(map[string]bool, len(files))
	for _, f := range files {
		active[f.Path] = true
	}
	for k := range s.previews {
		if !active[k] {
			delete(s.previews, k)
		}
	}
	s.previewMu.Unlock()
}

func (s *Server) pollLoop() {
	ticker := time.NewTicker(2 * time.Second)
	for range ticker.C {
		s.scan()
	}
}

func (s *Server) handleRecent(w http.ResponseWriter, r *http.Request) {
	n := 5
	if v, err := strconv.Atoi(r.URL.Query().Get("n")); err == nil && v > 0 && v <= 50 {
		n = v
	}
	clientExcludes := parseClientExcludes(r)

	s.mu.RLock()
	files := make([]FileInfo, 0, len(s.files))
	for _, f := range s.files {
		if !matchesClientExclude(f.Path, clientExcludes) {
			files = append(files, f)
		}
	}
	s.mu.RUnlock()

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime > files[j].ModTime
	})

	if len(files) > n {
		files = files[:n]
	}

	s.previewMu.Lock()
	for i := range files {
		cached, ok := s.previews[files[i].Path]
		if ok && cached.mtime == files[i].ModTime {
			files[i].Preview = cached.preview
		} else {
			p := extractPreview(filepath.Join(s.rootDir, files[i].Path), 150)
			s.previews[files[i].Path] = previewCache{mtime: files[i].ModTime, preview: p}
			files[i].Preview = p
		}
	}
	s.previewMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	clientExcludes := parseClientExcludes(r)

	s.mu.RLock()
	files := make([]FileInfo, 0, len(s.files))
	for _, f := range s.files {
		if !matchesClientExclude(f.Path, clientExcludes) {
			files = append(files, f)
		}
	}
	s.mu.RUnlock()

	root := &TreeNode{Name: "root", Children: []*TreeNode{}}
	dirMap := map[string]*TreeNode{"": root}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	for _, f := range files {
		parent := root
		if f.Dir != "" {
			parts := strings.Split(f.Dir, string(filepath.Separator))
			currentPath := ""
			for _, part := range parts {
				if currentPath == "" {
					currentPath = part
				} else {
					currentPath = currentPath + "/" + part
				}
				if _, ok := dirMap[currentPath]; !ok {
					node := &TreeNode{Name: part, Children: []*TreeNode{}}
					dirMap[currentPath] = node
					parent.Children = append(parent.Children, node)
				}
				parent = dirMap[currentPath]
			}
		}
		parent.Children = append(parent.Children, &TreeNode{
			Name:    f.Name,
			Path:    f.Path,
			IsFile:  true,
			ModTime: f.ModTime,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(root.Children)
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	reqPath := strings.TrimPrefix(r.URL.Path, "/files/")
	if reqPath == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	clean := filepath.Clean(reqPath)
	fullPath := filepath.Join(s.rootDir, clean)

	absPath, err := filepath.Abs(fullPath)
	if err != nil || !strings.HasPrefix(absPath, s.rootDir+string(filepath.Separator)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	lowerPath := strings.ToLower(absPath)
	allowed := false
	for _, ext := range []string{".html", ".md", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp"} {
		if strings.HasSuffix(lowerPath, ext) {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "file type not allowed", http.StatusForbidden)
		return
	}

	if strings.HasSuffix(lowerPath, ".md") {
		s.serveMarkdown(w, r, absPath)
		return
	}

	http.ServeFile(w, r, absPath)
}

func (s *Server) serveMarkdown(w http.ResponseWriter, r *http.Request, path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<style>
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif;max-width:980px;margin:0 auto;padding:32px 24px;color:#1f2328;line-height:1.6;font-size:16px}
body.thumb{max-width:none;margin:0;padding:12px;background:#f6f8fa;font-size:18px}
body.thumb h1{font-size:2.2em}
body.thumb h2{font-size:1.8em}
h1{font-size:2em;padding-bottom:.3em;border-bottom:1px solid #d1d9e0}
h2{font-size:1.5em;padding-bottom:.3em;border-bottom:1px solid #d1d9e0}
h3{font-size:1.25em}
a{color:#0969da;text-decoration:none}
a:hover{text-decoration:underline}
code{background:#eff1f3;padding:.2em .4em;border-radius:6px;font-size:85%}
pre{background:#f6f8fa;padding:16px;border-radius:6px;overflow-x:auto;line-height:1.45}
pre code{background:none;padding:0;font-size:85%}
blockquote{margin:0;padding:0 1em;color:#656d76;border-left:.25em solid #d1d9e0}
table{border-collapse:collapse;width:100%}
th,td{padding:6px 13px;border:1px solid #d1d9e0}
th{background:#f6f8fa;font-weight:600}
tr:nth-child(2n){background:#f6f8fa}
img{max-width:100%}
hr{height:.25em;padding:0;margin:24px 0;background:#d1d9e0;border:0}
ul,ol{padding-left:2em}
li+li{margin-top:.25em}
.task-list-item{list-style:none}
.task-list-item input{margin-right:.5em}
</style>
</head><body><div id="content"></div>
<script src="/static/marked.min.js"></script>
<script>
var b=atob('`))
	w.Write([]byte(base64.StdEncoding.EncodeToString(raw)))
	w.Write([]byte(`');var bytes=new Uint8Array(b.length);for(var i=0;i<b.length;i++)bytes[i]=b.charCodeAt(i);var text=new TextDecoder().decode(bytes);if(new URLSearchParams(location.search).has('thumb'))document.body.classList.add('thumb');document.getElementById('content').innerHTML=marked.parse(text);
</script></body></html>`))
}

func (s *Server) handleExcludes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp := map[string]interface{}{
		"defaults":     defaultExcludes,
		"cli":          s.cliExcludes,
		"fileDefaults": defaultFileExcludes,
		"fileCli":      s.cliFileExcludes,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, _ := frontendFS.ReadFile("frontend/index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func findCloudflared() string {
	if p, err := exec.LookPath("cloudflared"); err == nil {
		return p
	}
	if p, err := exec.LookPath("npx"); err == nil {
		return p
	}
	return ""
}

func startTunnel(port int) *exec.Cmd {
	bin := findCloudflared()
	if bin == "" {
		fmt.Fprintln(os.Stderr, "tunnel: cloudflared not found. Install it or run: npm install -g cloudflared")
		os.Exit(1)
	}

	var cmd *exec.Cmd
	if strings.HasSuffix(bin, "npx") || strings.HasSuffix(bin, "npx.cmd") {
		cmd = exec.Command(bin, "cloudflared", "tunnel", "--url", fmt.Sprintf("http://localhost:%d", port))
	} else {
		cmd = exec.Command(bin, "tunnel", "--url", fmt.Sprintf("http://localhost:%d", port))
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatalf("tunnel: %v", err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("tunnel: failed to start cloudflared: %v", err)
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "trycloudflare.com") || strings.Contains(line, "cfargotunnel.com") {
				for _, word := range strings.Fields(line) {
					if strings.HasPrefix(word, "https://") {
						fmt.Printf("\n  Share this link: %s\n\n", word)
						fmt.Println("  Anyone with this link can view your reports.")
						fmt.Println("  This link expires when you stop viewllm — a new one is created each time.")
						fmt.Println("  Learn more: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/")
						return
					}
				}
			}
		}
	}()

	return cmd
}

func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}

func isSSH() bool {
	return os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != ""
}

func getLANIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil && !ipnet.IP.IsLoopback() {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func getPublicIP() string {
	if isWSL() {
		return ""
	}

	ch := make(chan string, 1)
	send := func(ip string) {
		select {
		case ch <- ip:
		default:
		}
	}

	// AWS EC2 metadata (IMDSv1, then v2 fallback)
	go func() {
		client := &http.Client{Timeout: 150 * time.Millisecond}
		resp, err := client.Get("http://169.254.169.254/latest/meta-data/public-ipv4")
		if err != nil {
			return
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 {
			if ip := strings.TrimSpace(string(body)); net.ParseIP(ip) != nil {
				send(ip)
				return
			}
		}
		if resp.StatusCode == 401 {
			tokenReq, _ := http.NewRequest("PUT", "http://169.254.169.254/latest/api/token", nil)
			tokenReq.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "60")
			tr, err := client.Do(tokenReq)
			if err != nil {
				return
			}
			token, _ := io.ReadAll(tr.Body)
			tr.Body.Close()
			if tr.StatusCode != 200 {
				return
			}
			req, _ := http.NewRequest("GET", "http://169.254.169.254/latest/meta-data/public-ipv4", nil)
			req.Header.Set("X-aws-ec2-metadata-token", strings.TrimSpace(string(token)))
			r2, err := client.Do(req)
			if err != nil {
				return
			}
			b2, _ := io.ReadAll(r2.Body)
			r2.Body.Close()
			if r2.StatusCode == 200 {
				if ip := strings.TrimSpace(string(b2)); net.ParseIP(ip) != nil {
					send(ip)
				}
			}
		}
	}()

	// GCP metadata
	go func() {
		client := &http.Client{Timeout: 150 * time.Millisecond}
		req, _ := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip", nil)
		req.Header.Set("Metadata-Flavor", "Google")
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 {
			if ip := strings.TrimSpace(string(body)); net.ParseIP(ip) != nil {
				send(ip)
			}
		}
	}()

	// Azure IMDS
	go func() {
		client := &http.Client{Timeout: 150 * time.Millisecond}
		req, _ := http.NewRequest("GET", "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2021-02-01&format=text", nil)
		req.Header.Set("Metadata", "true")
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 {
			if ip := strings.TrimSpace(string(body)); net.ParseIP(ip) != nil {
				send(ip)
			}
		}
	}()

	select {
	case ip := <-ch:
		return ip
	case <-time.After(200 * time.Millisecond):
		return ""
	}
}

func printAccessHints(port int, publicIP string) {
	lanIP := getLANIP()

	fmt.Println()
	if publicIP != "" {
		fmt.Printf("  Share: http://%s:%d\n", publicIP, port)
		if lanIP != "" && lanIP != publicIP {
			fmt.Printf("  Local: http://%s:%d\n", lanIP, port)
		}
	} else if lanIP != "" {
		fmt.Printf("  Open: http://%s:%d\n", lanIP, port)
		fmt.Printf("  To share over the internet, use -tunnel\n")
	} else {
		fmt.Printf("  Open: http://localhost:%d\n", port)
		fmt.Printf("  To share over the internet, use -tunnel\n")
	}
	fmt.Println()
}

func main() {
	port := 8090
	tunnel := false
	var dir string
	var cliExcludes []string
	var cliFileExcludes []string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		if args[i] == "-p" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%d", &port)
			i++
		} else if args[i] == "-exclude" && i+1 < len(args) {
			cliExcludes = append(cliExcludes, args[i+1])
			i++
		} else if args[i] == "-exclude-file" && i+1 < len(args) {
			cliFileExcludes = append(cliFileExcludes, args[i+1])
			i++
		} else if args[i] == "-tunnel" {
			tunnel = true
		} else if len(args[i]) > 0 && args[i][0] != '-' {
			dir = args[i]
		}
	}

	if dir == "" {
		dir = "."
	}

	// Start public IP detection early (runs concurrently with setup)
	publicIPCh := make(chan string, 1)
	go func() { publicIPCh <- getPublicIP() }()

	rootDir, err := filepath.Abs(dir)
	if err != nil {
		log.Fatalf("invalid path: %v", err)
	}

	info, err := os.Stat(rootDir)
	if err != nil || !info.IsDir() {
		log.Fatalf("not a directory: %s", rootDir)
	}

	srv := NewServer(rootDir, cliExcludes, cliFileExcludes)

	http.HandleFunc("/", srv.handleIndex)
	http.HandleFunc("/api/recent", srv.handleRecent)
	http.HandleFunc("/api/tree", srv.handleTree)
	http.HandleFunc("/api/excludes", srv.handleExcludes)
	http.HandleFunc("/files/", srv.handleFiles)
	http.HandleFunc("/static/marked.min.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Write(markedJS)
	})

	userSetPort := false
	for _, a := range os.Args[1:] {
		if a == "-p" {
			userSetPort = true
			break
		}
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil && !userSetPort {
		for try := port + 1; try < port+20; try++ {
			ln, err = net.Listen("tcp", fmt.Sprintf(":%d", try))
			if err == nil {
				port = try
				break
			}
		}
	}
	if err != nil {
		log.Fatalf("port %d is in use", port)
	}

	publicIP := <-publicIPCh

	if !tunnel {
		fmt.Printf("viewllm serving %s\n", rootDir)
		printAccessHints(port, publicIP)
	} else {
		fmt.Printf("viewllm serving %s — starting tunnel (powered by Cloudflare)...\n", rootDir)
	}

	var tunnelCmd *exec.Cmd
	if tunnel {
		go func() {
			log.Fatal(http.Serve(ln, nil))
		}()
		tunnelCmd = startTunnel(port)

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		fmt.Println("\nShutting down...")
		if tunnelCmd.Process != nil {
			tunnelCmd.Process.Kill()
		}
	} else {
		log.Fatal(http.Serve(ln, nil))
	}
}
