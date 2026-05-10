package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
}

type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path,omitempty"`
	Children []*TreeNode `json:"children,omitempty"`
	IsFile   bool        `json:"isFile"`
	ModTime  int64       `json:"mtime,omitempty"`
}

type Server struct {
	rootDir string
	mu      sync.RWMutex
	files   []FileInfo
}

func NewServer(rootDir string) *Server {
	s := &Server{rootDir: rootDir}
	s.scan()
	go s.pollLoop()
	return s
}

func (s *Server) scan() {
	var files []FileInfo
	filepath.WalkDir(s.rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
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
}

func (s *Server) pollLoop() {
	ticker := time.NewTicker(2 * time.Second)
	for range ticker.C {
		s.scan()
	}
}

func (s *Server) handleRecent(w http.ResponseWriter, r *http.Request) {
	n := 5
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
	if strings.Contains(clean, "..") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	fullPath := filepath.Join(s.rootDir, clean)

	if !strings.HasSuffix(strings.ToLower(fullPath), ".html") {
		http.Error(w, "only HTML files are served", http.StatusForbidden)
		return
	}

	http.ServeFile(w, r, fullPath)
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

func main() {
	port := 8090
	var dir string

	// Parse args manually to allow flags in any position: viewllm ./dir -p 8095
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		if args[i] == "-p" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%d", &port)
			i++
		} else if args[i][0] != '-' {
			dir = args[i]
		}
	}

	if dir == "" {
		fmt.Fprintf(os.Stderr, "Usage: viewllm <directory> [-p port]\n")
		os.Exit(1)
	}

	rootDir, err := filepath.Abs(dir)
	if err != nil {
		log.Fatalf("invalid path: %v", err)
	}

	info, err := os.Stat(rootDir)
	if err != nil || !info.IsDir() {
		log.Fatalf("not a directory: %s", rootDir)
	}

	srv := NewServer(rootDir)

	http.HandleFunc("/", srv.handleIndex)
	http.HandleFunc("/api/recent", srv.handleRecent)
	http.HandleFunc("/api/tree", srv.handleTree)
	http.HandleFunc("/files/", srv.handleFiles)

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("viewllm serving %s on http://0.0.0.0%s\n", rootDir, addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
