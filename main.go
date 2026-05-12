package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"bufio"
	"net"
	"io/fs"
	"log"
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

type FileInfo struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Dir     string `json:"dir"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mtime"`
	Preview string `json:"preview,omitempty"`
}

func extractPreview(path string, maxLen int) string {
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

	text := strings.TrimSpace(b.String())
	if len(text) > maxLen {
		text = text[:maxLen]
		if j := strings.LastIndex(text, " "); j > maxLen/2 {
			text = text[:j]
		}
		text += "..."
	}
	return text
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
}

type previewCache struct {
	mtime   int64
	preview string
}

type Server struct {
	rootDir      string
	mu           sync.RWMutex
	files        []FileInfo
	previewMu    sync.Mutex
	previews     map[string]previewCache
	cliExcludes  []string
	userExcludes []string
}

func NewServer(rootDir string, cliExcludes []string) *Server {
	s := &Server{rootDir: rootDir, cliExcludes: cliExcludes, previews: make(map[string]previewCache)}
	s.scan()
	go s.pollLoop()
	return s
}

func (s *Server) isExcluded(name string) bool {
	s.mu.RLock()
	userExcludes := make([]string, len(s.userExcludes))
	copy(userExcludes, s.userExcludes)
	s.mu.RUnlock()

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
	for _, pat := range userExcludes {
		if strings.EqualFold(name, pat) {
			return true
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
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".html") {
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
	s.mu.RLock()
	files := make([]FileInfo, len(s.files))
	copy(files, s.files)
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
	s.mu.RLock()
	files := make([]FileInfo, len(s.files))
	copy(files, s.files)
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

	if !strings.HasSuffix(strings.ToLower(absPath), ".html") {
		http.Error(w, "only HTML files are served", http.StatusForbidden)
		return
	}

	http.ServeFile(w, r, absPath)
}

func (s *Server) handleExcludes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		userExcludes := make([]string, len(s.userExcludes))
		copy(userExcludes, s.userExcludes)
		s.mu.RUnlock()

		resp := map[string][]string{
			"defaults": defaultExcludes,
			"cli":      s.cliExcludes,
			"user":     userExcludes,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	case http.MethodPost:
		var req struct {
			Action  string `json:"action"`
			Pattern string `json:"pattern"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		req.Pattern = strings.TrimSpace(req.Pattern)
		if req.Pattern == "" {
			http.Error(w, "pattern required", http.StatusBadRequest)
			return
		}

		s.mu.Lock()
		switch req.Action {
		case "add":
			for _, p := range s.userExcludes {
				if strings.EqualFold(p, req.Pattern) {
					s.mu.Unlock()
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]string{"status": "exists"})
					return
				}
			}
			s.userExcludes = append(s.userExcludes, req.Pattern)
		case "remove":
			filtered := s.userExcludes[:0]
			for _, p := range s.userExcludes {
				if !strings.EqualFold(p, req.Pattern) {
					filtered = append(filtered, p)
				}
			}
			s.userExcludes = filtered
		default:
			s.mu.Unlock()
			http.Error(w, "action must be add or remove", http.StatusBadRequest)
			return
		}
		s.mu.Unlock()

		s.scan()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
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

func printAccessHints(port int) {
	lanIP := getLANIP()

	fmt.Println()
	if lanIP != "" {
		fmt.Printf("  Open: http://%s:%d\n", lanIP, port)
	} else {
		fmt.Printf("  Open: http://localhost:%d\n", port)
	}
	fmt.Printf("  To share over the internet, use -tunnel\n")
	fmt.Println()
}

func main() {
	port := 8090
	tunnel := false
	var dir string
	var cliExcludes []string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		if args[i] == "-p" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%d", &port)
			i++
		} else if args[i] == "-exclude" && i+1 < len(args) {
			cliExcludes = append(cliExcludes, args[i+1])
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

	rootDir, err := filepath.Abs(dir)
	if err != nil {
		log.Fatalf("invalid path: %v", err)
	}

	info, err := os.Stat(rootDir)
	if err != nil || !info.IsDir() {
		log.Fatalf("not a directory: %s", rootDir)
	}

	srv := NewServer(rootDir, cliExcludes)

	http.HandleFunc("/", srv.handleIndex)
	http.HandleFunc("/api/recent", srv.handleRecent)
	http.HandleFunc("/api/tree", srv.handleTree)
	http.HandleFunc("/api/excludes", srv.handleExcludes)
	http.HandleFunc("/files/", srv.handleFiles)

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

	if !tunnel {
		fmt.Printf("viewllm serving %s\n", rootDir)
		printAccessHints(port)
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
