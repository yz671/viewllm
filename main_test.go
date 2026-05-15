package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTruncateAtWord(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		maxLen int
		want   string
	}{
		{"short text", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"empty string", "", 10, ""},
		{"truncate at word", "hello world foo", 11, "hello world..."},
		{"truncate no space", "abcdefghijklmnop", 10, "abcdefghij..."},
		{"space in first half only", "ab cdefghijklmnop", 10, "ab cdefghi..."},
		{"space in second half", "abcdef ghijklmnop", 10, "abcdef..."},
		{"maxLen 1", "hello", 1, "h..."},
		{"maxLen 0", "hello", 0, "..."},
		{"multiple spaces", "a b c d e f g h i j", 10, "a b c d e..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateAtWord(tt.text, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateAtWord(%q, %d) = %q, want %q", tt.text, tt.maxLen, got, tt.want)
			}
		})
	}
}

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractMarkdownPreview(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		content string
		maxLen  int
		want    string
	}{
		{
			"basic text",
			"# Title\n\nSome body text here.",
			100,
			"Some body text here.",
		},
		{
			"skips headers and frontmatter",
			"---\ntitle: test\n---\n# Heading\n## Subheading\nActual content.",
			100,
			"title: test Actual content.",
		},
		{
			"skips code fences",
			"# Title\n```python\nprint('hello')\n```\nAfter code.",
			100,
			"print('hello') After code.",
		},
		{
			"skips HTML and images",
			"<div>html</div>\n![alt](img.png)\nVisible text.",
			100,
			"Visible text.",
		},
		{
			"empty file",
			"",
			100,
			"",
		},
		{
			"only headers",
			"# Title\n## Section\n### Sub",
			100,
			"",
		},
		{
			"truncates long text",
			"This is a long sentence that exceeds the max length limit.",
			20,
			"This is a long...",
		},
		{
			"multiple body lines",
			"# Title\nLine one.\nLine two.\nLine three.",
			100,
			"Line one. Line two. Line three.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemp(t, dir, tt.name+".md", tt.content)
			got := extractMarkdownPreview(path, tt.maxLen)
			if got != tt.want {
				t.Errorf("extractMarkdownPreview() = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("nonexistent file", func(t *testing.T) {
		got := extractMarkdownPreview("/nonexistent/path.md", 100)
		if got != "" {
			t.Errorf("expected empty string for nonexistent file, got %q", got)
		}
	})
}

func TestExtractPreview(t *testing.T) {
	dir := t.TempDir()

	t.Run("delegates md to extractMarkdownPreview", func(t *testing.T) {
		path := writeTemp(t, dir, "test.md", "# Title\nBody text.")
		got := extractPreview(path, 100)
		if got != "Body text." {
			t.Errorf("extractPreview(.md) = %q, want %q", got, "Body text.")
		}
	})

	t.Run("extracts from html body", func(t *testing.T) {
		html := `<html><head><title>Test</title></head><body><h1>Hello</h1><p>World</p></body></html>`
		path := writeTemp(t, dir, "test.html", html)
		got := extractPreview(path, 100)
		if !strings.Contains(got, "Hello") || !strings.Contains(got, "World") {
			t.Errorf("extractPreview(html) = %q, expected to contain 'Hello' and 'World'", got)
		}
	})

	t.Run("strips script and style", func(t *testing.T) {
		html := `<body><script>var x = 1;</script><style>.a{}</style><p>Visible</p></body>`
		path := writeTemp(t, dir, "strip.html", html)
		got := extractPreview(path, 100)
		if strings.Contains(got, "var x") || strings.Contains(got, ".a{}") {
			t.Errorf("extractPreview should strip script/style, got %q", got)
		}
		if !strings.Contains(got, "Visible") {
			t.Errorf("extractPreview should contain 'Visible', got %q", got)
		}
	})

	t.Run("handles html entities", func(t *testing.T) {
		html := `<body><p>Hello&amp;World</p></body>`
		path := writeTemp(t, dir, "entities.html", html)
		got := extractPreview(path, 100)
		if !strings.Contains(got, "Hello") || !strings.Contains(got, "World") {
			t.Errorf("extractPreview should handle entities, got %q", got)
		}
	})

	t.Run("collapses whitespace", func(t *testing.T) {
		html := `<body><p>Hello   World</p><p>Foo
Bar</p></body>`
		path := writeTemp(t, dir, "ws.html", html)
		got := extractPreview(path, 100)
		if strings.Contains(got, "  ") {
			t.Errorf("extractPreview should collapse whitespace, got %q", got)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		got := extractPreview("/nonexistent/path.html", 100)
		if got != "" {
			t.Errorf("expected empty for nonexistent file, got %q", got)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := writeTemp(t, dir, "empty.html", "")
		got := extractPreview(path, 100)
		if got != "" {
			t.Errorf("expected empty for empty file, got %q", got)
		}
	})

	t.Run("no body tag", func(t *testing.T) {
		html := `<p>No body tag content</p>`
		path := writeTemp(t, dir, "nobody.html", html)
		got := extractPreview(path, 100)
		if !strings.Contains(got, "No body tag content") {
			t.Errorf("extractPreview should work without body tag, got %q", got)
		}
	})

	t.Run("case insensitive body tag", func(t *testing.T) {
		html := `<BODY><P>Upper case</P></BODY>`
		path := writeTemp(t, dir, "upper.html", html)
		got := extractPreview(path, 100)
		if !strings.Contains(got, "Upper case") {
			t.Errorf("extractPreview should handle uppercase tags, got %q", got)
		}
	})
}

func TestIsExcluded(t *testing.T) {
	s := &Server{
		cliExcludes: []string{"logs"},
		previews:    make(map[string]previewCache),
	}

	tests := []struct {
		name string
		dir  string
		want bool
	}{
		{"default exclude", "node_modules", true},
		{"default exclude .git", ".git", true},
		{"default exclude venv", "venv", true},
		{"case insensitive default", "Node_Modules", true},
		{"cli exclude", "logs", true},
		{"cli exclude case", "LOGS", true},
		{"not excluded", "src", false},
		{"not excluded similar", "node_module", false},
		{"empty name", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.isExcluded(tt.dir)
			if got != tt.want {
				t.Errorf("isExcluded(%q) = %v, want %v", tt.dir, got, tt.want)
			}
		})
	}
}

func TestParseClientExcludes(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		wantFolders int
		wantFiles   int
	}{
		{"empty", "", 0, 0},
		{"single folder", "?excludes=logs", 1, 0},
		{"multiple folders", "?excludes=logs,tmp,data", 3, 0},
		{"with spaces", "?excludes=logs%2C%20tmp", 2, 0},
		{"file exclude", "?excludes=file:CLAUDE.md", 0, 1},
		{"mixed", "?excludes=logs,file:CLAUDE.md,tmp", 2, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/tree"+tt.query, nil)
			got := parseClientExcludes(req)
			if len(got.folders) != tt.wantFolders {
				t.Errorf("parseClientExcludes() folders = %d, want %d", len(got.folders), tt.wantFolders)
			}
			if len(got.files) != tt.wantFiles {
				t.Errorf("parseClientExcludes() files = %d, want %d", len(got.files), tt.wantFiles)
			}
		})
	}
}

func TestMatchesClientExclude(t *testing.T) {
	tests := []struct {
		name string
		path string
		ce   clientExcludes
		want bool
	}{
		{"no excludes", "a/b/file.html", clientExcludes{}, false},
		{"match dir", "logs/file.html", clientExcludes{folders: []string{"logs"}}, true},
		{"match nested", "a/logs/file.html", clientExcludes{folders: []string{"logs"}}, true},
		{"case insensitive", "a/LOGS/file.html", clientExcludes{folders: []string{"logs"}}, true},
		{"no match", "a/b/file.html", clientExcludes{folders: []string{"logs"}}, false},
		{"root file", "file.html", clientExcludes{folders: []string{"logs"}}, false},
		{"partial no match", "a/xxlogsxx/file.html", clientExcludes{folders: []string{"logs"}}, false},
		{"file exclude match", "a/b/CLAUDE.md", clientExcludes{files: []string{"CLAUDE.md"}}, true},
		{"file exclude case insensitive", "a/b/claude.md", clientExcludes{files: []string{"CLAUDE.md"}}, true},
		{"file exclude no match", "a/b/README.md", clientExcludes{files: []string{"CLAUDE.md"}}, false},
		{"mixed match folder", "logs/file.html", clientExcludes{folders: []string{"logs"}, files: []string{"CLAUDE.md"}}, true},
		{"mixed match file", "a/CLAUDE.md", clientExcludes{folders: []string{"logs"}, files: []string{"CLAUDE.md"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesClientExclude(tt.path, tt.ce)
			if got != tt.want {
				t.Errorf("matchesClientExclude(%q, %v) = %v, want %v", tt.path, tt.ce, got, tt.want)
			}
		})
	}
}

func TestScan(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "report.html", "<html><body>test</body></html>")
	writeTemp(t, dir, "notes.md", "# Notes")
	writeTemp(t, dir, "code.go", "package main")
	writeTemp(t, dir, "data.json", "{}")
	writeTemp(t, dir, "sub/nested.html", "<html></html>")
	writeTemp(t, dir, "node_modules/pkg.html", "<html></html>")
	writeTemp(t, dir, ".git/config.html", "<html></html>")
	writeTemp(t, dir, "CLAUDE.md", "# Claude config")
	writeTemp(t, dir, "sub/CLAUDE.md", "# Nested claude config")

	s := &Server{rootDir: dir, previews: make(map[string]previewCache)}
	s.scan()

	s.mu.RLock()
	files := s.files
	s.mu.RUnlock()

	paths := make(map[string]bool)
	for _, f := range files {
		paths[f.Path] = true
	}

	if !paths["report.html"] {
		t.Error("scan should find report.html")
	}
	if !paths["notes.md"] {
		t.Error("scan should find notes.md")
	}
	if !paths[filepath.Join("sub", "nested.html")] {
		t.Error("scan should find sub/nested.html")
	}
	if paths["code.go"] {
		t.Error("scan should not find .go files")
	}
	if paths["data.json"] {
		t.Error("scan should not find .json files")
	}
	if paths[filepath.Join("node_modules", "pkg.html")] {
		t.Error("scan should exclude node_modules")
	}
	if paths[filepath.Join(".git", "config.html")] {
		t.Error("scan should exclude .git")
	}
	if paths["CLAUDE.md"] {
		t.Error("scan should exclude CLAUDE.md by default file exclude")
	}
	if paths[filepath.Join("sub", "CLAUDE.md")] {
		t.Error("scan should exclude sub/CLAUDE.md by default file exclude")
	}

	t.Run("dir field", func(t *testing.T) {
		for _, f := range files {
			if f.Name == "report.html" && f.Dir != "" {
				t.Errorf("root file should have empty Dir, got %q", f.Dir)
			}
			if f.Name == "nested.html" && f.Dir != "sub" {
				t.Errorf("nested file Dir should be 'sub', got %q", f.Dir)
			}
		}
	})

	t.Run("stale preview cleanup", func(t *testing.T) {
		s.previewMu.Lock()
		s.previews["deleted.html"] = previewCache{mtime: 1, preview: "old"}
		s.previewMu.Unlock()

		s.scan()

		s.previewMu.Lock()
		_, exists := s.previews["deleted.html"]
		s.previewMu.Unlock()
		if exists {
			t.Error("scan should clean up preview cache for deleted files")
		}
	})
}

func newTestServer(t *testing.T, dir string) *Server {
	t.Helper()
	s := &Server{rootDir: dir, previews: make(map[string]previewCache)}
	s.scan()
	return s
}

func TestHandleFilesPathTraversal(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "safe.html", "<html>safe</html>")

	absDir, _ := filepath.Abs(dir)
	s := &Server{rootDir: absDir, previews: make(map[string]previewCache)}

	tests := []struct {
		name string
		path string
		code int
	}{
		{"valid file", "/files/safe.html", 200},
		{"path traversal", "/files/../../../etc/passwd", 403},
		{"path traversal dotdot", "/files/../../etc/hosts", 403},
		{"empty path", "/files/", 404},
		{"disallowed extension", "/files/safe.go", 403},
		{"nonexistent html", "/files/missing.html", 404},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			s.handleFiles(w, req)
			if w.Code != tt.code {
				t.Errorf("%s: got status %d, want %d", tt.path, w.Code, tt.code)
			}
		})
	}
}

func TestHandleFilesAllowedExtensions(t *testing.T) {
	dir := t.TempDir()
	absDir, _ := filepath.Abs(dir)

	extensions := map[string]bool{
		".html": true,
		".md":   true,
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".gif":  true,
		".svg":  true,
		".webp": true,
		".go":   false,
		".js":   false,
		".css":  false,
		".txt":  false,
		".exe":  false,
		".sh":   false,
	}

	for ext, allowed := range extensions {
		name := "test" + ext
		writeTemp(t, dir, name, "content")

		s := &Server{rootDir: absDir, previews: make(map[string]previewCache)}
		req := httptest.NewRequest("GET", "/files/"+name, nil)
		w := httptest.NewRecorder()
		s.handleFiles(w, req)

		if allowed && w.Code == 403 {
			t.Errorf("extension %s should be allowed, got 403", ext)
		}
		if !allowed && w.Code != 403 {
			t.Errorf("extension %s should be forbidden, got %d", ext, w.Code)
		}
	}
}

func TestHandleRecent(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "a.html", "<body>Alpha</body>")
	writeTemp(t, dir, "b.html", "<body>Beta</body>")
	writeTemp(t, dir, "c.html", "<body>Gamma</body>")

	s := newTestServer(t, dir)

	t.Run("default n=5", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/recent", nil)
		w := httptest.NewRecorder()
		s.handleRecent(w, req)

		if w.Code != 200 {
			t.Fatalf("got status %d", w.Code)
		}
		var files []FileInfo
		json.NewDecoder(w.Body).Decode(&files)
		if len(files) != 3 {
			t.Errorf("expected 3 files, got %d", len(files))
		}
	})

	t.Run("custom n", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/recent?n=2", nil)
		w := httptest.NewRecorder()
		s.handleRecent(w, req)

		var files []FileInfo
		json.NewDecoder(w.Body).Decode(&files)
		if len(files) != 2 {
			t.Errorf("expected 2 files, got %d", len(files))
		}
	})

	t.Run("invalid n falls back", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/recent?n=abc", nil)
		w := httptest.NewRecorder()
		s.handleRecent(w, req)

		var files []FileInfo
		json.NewDecoder(w.Body).Decode(&files)
		if len(files) != 3 {
			t.Errorf("expected 3 files with invalid n, got %d", len(files))
		}
	})

	t.Run("n exceeds limit", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/recent?n=100", nil)
		w := httptest.NewRecorder()
		s.handleRecent(w, req)

		var files []FileInfo
		json.NewDecoder(w.Body).Decode(&files)
		if len(files) != 3 {
			t.Errorf("expected 3 files when n>50, got %d", len(files))
		}
	})

	t.Run("has previews", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/recent?n=3", nil)
		w := httptest.NewRecorder()
		s.handleRecent(w, req)

		var files []FileInfo
		json.NewDecoder(w.Body).Decode(&files)
		hasPreview := false
		for _, f := range files {
			if f.Preview != "" {
				hasPreview = true
			}
		}
		if !hasPreview {
			t.Error("expected at least one file to have a preview")
		}
	})

	t.Run("sorted by mtime descending", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/recent?n=10", nil)
		w := httptest.NewRecorder()
		s.handleRecent(w, req)

		var files []FileInfo
		json.NewDecoder(w.Body).Decode(&files)
		for i := 1; i < len(files); i++ {
			if files[i].ModTime > files[i-1].ModTime {
				t.Error("files should be sorted by mtime descending")
			}
		}
	})
}

func TestHandleTree(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "root.html", "<html></html>")
	writeTemp(t, dir, "docs/guide.html", "<html></html>")
	writeTemp(t, dir, "docs/api.md", "# API")
	writeTemp(t, dir, "docs/sub/deep.html", "<html></html>")

	s := newTestServer(t, dir)

	req := httptest.NewRequest("GET", "/api/tree", nil)
	w := httptest.NewRecorder()
	s.handleTree(w, req)

	if w.Code != 200 {
		t.Fatalf("got status %d", w.Code)
	}

	var tree []*TreeNode
	json.NewDecoder(w.Body).Decode(&tree)

	if len(tree) == 0 {
		t.Fatal("tree should not be empty")
	}

	var findNode func([]*TreeNode, string) *TreeNode
	findNode = func(nodes []*TreeNode, name string) *TreeNode {
		for _, n := range nodes {
			if n.Name == name {
				return n
			}
			if found := findNode(n.Children, name); found != nil {
				return found
			}
		}
		return nil
	}

	rootFile := findNode(tree, "root.html")
	if rootFile == nil || !rootFile.IsFile {
		t.Error("should find root.html as a file node")
	}

	docsDir := findNode(tree, "docs")
	if docsDir == nil || docsDir.IsFile {
		t.Error("should find docs as a directory node")
	}
	if docsDir != nil && len(docsDir.Children) < 2 {
		t.Errorf("docs should have at least 2 children, got %d", len(docsDir.Children))
	}

	deepFile := findNode(tree, "deep.html")
	if deepFile == nil {
		t.Error("should find deeply nested file")
	}
}

func TestHandleExcludes(t *testing.T) {
	dir := t.TempDir()
	s := &Server{
		rootDir:     dir,
		cliExcludes: []string{"cli_pattern"},
		previews:    make(map[string]previewCache),
	}

	t.Run("GET returns defaults and cli", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/excludes", nil)
		w := httptest.NewRecorder()
		s.handleExcludes(w, req)

		var data map[string][]string
		json.NewDecoder(w.Body).Decode(&data)

		if len(data["defaults"]) == 0 {
			t.Error("defaults should not be empty")
		}
		if len(data["cli"]) != 1 || data["cli"][0] != "cli_pattern" {
			t.Errorf("cli should be [cli_pattern], got %v", data["cli"])
		}
	})

	t.Run("POST not allowed", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/excludes", nil)
		w := httptest.NewRecorder()
		s.handleExcludes(w, req)

		if w.Code != 405 {
			t.Errorf("expected 405 for POST, got %d", w.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/api/excludes", nil)
		w := httptest.NewRecorder()
		s.handleExcludes(w, req)

		if w.Code != 405 {
			t.Errorf("expected 405 for PUT, got %d", w.Code)
		}
	})
}

func TestServeMarkdown(t *testing.T) {
	dir := t.TempDir()
	content := "# Hello\n\nWorld with **bold** and `code`."
	path := writeTemp(t, dir, "test.md", content)

	absDir, _ := filepath.Abs(dir)
	s := &Server{rootDir: absDir, previews: make(map[string]previewCache)}

	t.Run("returns html content type", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/files/test.md", nil)
		w := httptest.NewRecorder()
		s.serveMarkdown(w, req, path)

		if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
			t.Errorf("Content-Type = %q, want text/html; charset=utf-8", ct)
		}
	})

	t.Run("contains base64 encoded content", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/files/test.md", nil)
		w := httptest.NewRecorder()
		s.serveMarkdown(w, req, path)

		body := w.Body.String()
		b64 := base64.StdEncoding.EncodeToString([]byte(content))
		if !strings.Contains(body, b64) {
			t.Error("response should contain base64-encoded markdown content")
		}
	})

	t.Run("references static marked.js", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/files/test.md", nil)
		w := httptest.NewRecorder()
		s.serveMarkdown(w, req, path)

		body := w.Body.String()
		if !strings.Contains(body, `/static/marked.min.js`) {
			t.Error("response should reference /static/marked.min.js")
		}
	})

	t.Run("contains thumb detection", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/files/test.md", nil)
		w := httptest.NewRecorder()
		s.serveMarkdown(w, req, path)

		body := w.Body.String()
		if !strings.Contains(body, "thumb") {
			t.Error("response should contain thumb class detection")
		}
	})

	t.Run("nonexistent file returns 404", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/files/missing.md", nil)
		w := httptest.NewRecorder()
		s.serveMarkdown(w, req, filepath.Join(dir, "missing.md"))

		if w.Code != 404 {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("utf8 content roundtrips", func(t *testing.T) {
		utf8Content := "# Héllo Wörld\n\nArrows → and dots · everywhere"
		utf8Path := writeTemp(t, dir, "utf8.md", utf8Content)

		req := httptest.NewRequest("GET", "/files/utf8.md", nil)
		w := httptest.NewRecorder()
		s.serveMarkdown(w, req, utf8Path)

		body := w.Body.String()
		b64 := base64.StdEncoding.EncodeToString([]byte(utf8Content))
		if !strings.Contains(body, b64) {
			t.Error("UTF-8 content should be correctly base64 encoded in response")
		}
	})
}

func TestHandleIndex(t *testing.T) {
	s := &Server{previews: make(map[string]previewCache)}

	t.Run("root path serves html", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		s.handleIndex(w, req)

		if w.Code != 200 {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
			t.Errorf("Content-Type = %q", ct)
		}
	})

	t.Run("non-root path returns 404", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/other", nil)
		w := httptest.NewRecorder()
		s.handleIndex(w, req)

		if w.Code != 404 {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}

func TestIsSSH(t *testing.T) {
	os.Unsetenv("SSH_CONNECTION")
	os.Unsetenv("SSH_CLIENT")

	if isSSH() {
		t.Error("isSSH should be false when env vars are unset")
	}

	os.Setenv("SSH_CONNECTION", "1.2.3.4 1234 5.6.7.8 22")
	if !isSSH() {
		t.Error("isSSH should be true when SSH_CONNECTION is set")
	}
	os.Unsetenv("SSH_CONNECTION")

	os.Setenv("SSH_CLIENT", "1.2.3.4 1234 22")
	if !isSSH() {
		t.Error("isSSH should be true when SSH_CLIENT is set")
	}
	os.Unsetenv("SSH_CLIENT")
}

func TestDefaultExcludes(t *testing.T) {
	expected := []string{
		"venv", ".venv", "node_modules", ".git", "__pycache__",
		".tox", ".mypy_cache", ".pytest_cache", ".eggs", "dist",
		"build", ".next", ".nuxt", ".cache",
		".claude", ".codex", ".aider", ".cursor",
		".vscode-server", ".idea",
		"target", "vendor", "coverage",
	}
	if len(defaultExcludes) != len(expected) {
		t.Errorf("defaultExcludes has %d entries, expected %d", len(defaultExcludes), len(expected))
	}
	for _, e := range expected {
		found := false
		for _, d := range defaultExcludes {
			if d == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("defaultExcludes missing %q", e)
		}
	}
}

func TestStaticMarkedJS(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Write(markedJS)
	}

	req := httptest.NewRequest("GET", "/static/marked.min.js", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/javascript" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control = %q, expected immutable", cc)
	}
	if w.Body.Len() == 0 {
		t.Error("marked.js response should not be empty")
	}
	if !strings.Contains(w.Body.String(), "marked") {
		t.Error("response should contain marked library code")
	}
}
