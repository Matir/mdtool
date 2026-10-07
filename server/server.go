package server

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Matir/mdtool/converter"
	"github.com/fsnotify/fsnotify"
)

var dirListingTemplate = template.Must(template.New("dirListing").Parse(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        :root { color-scheme: dark; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "Noto Sans", Helvetica, Arial, sans-serif; padding: 2em; line-height: 1.6; max-width: 860px; margin: auto; background-color: #0d1117; color: #e6edf3; }
        h1 { color: #f0f6fc; border-bottom: 1px solid #21262d; padding-bottom: 0.3em; }
        ul { list-style: none; padding: 0; }
        li { border-bottom: 1px solid #21262d; padding: 0.5em 0; }
        a { text-decoration: none; color: #58a6ff; }
        a:visited { color: #a371f7; }
        a:hover { color: #79c0ff; text-decoration: underline; }
        .dir { font-weight: bold; }
    </style>
</head>
<body>
<h1>Index of {{ .URLPath }}</h1>
<ul>
    {{- if .ShowParent }}
    <li><a href="..">..</a></li>
    {{- end }}
    {{- range .Items }}
    <li><a href="{{ .Href }}"{{ if .IsDir }} class="dir"{{ end }}>{{ .Name }}</a></li>
    {{- end }}
</ul>
</body>
</html>`))

type dirListingItem struct {
	Name  string
	Href  string
	IsDir bool
}

type dirListingData struct {
	URLPath    string
	ShowParent bool
	Items      []dirListingItem
}

// Server holds the configuration for the web server.
type Server struct {
	Dir       string
	Listen    string
	OnlyMD    bool
	Converter *converter.Converter
	Watch     bool

	mu      sync.Mutex
	clients []chan struct{}
}

// New returns a new Server with the specified options.
func New(dir, listen string, onlyMD bool, c *converter.Converter) *Server {
	if dir == "" {
		dir = "."
	}
	if listen == "" {
		listen = "127.0.0.1:7768"
	}
	return &Server{
		Dir:       dir,
		Listen:    listen,
		OnlyMD:    onlyMD,
		Converter: c,
	}
}

// Serve starts the web server.
func (s *Server) Serve() error {
	if s.Watch {
		go s.watchFiles()
	}

	s.Converter.EmbedAssets = false
	mux := http.NewServeMux()
	mux.HandleFunc("/_mdtool/mermaid.min.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fmt.Fprint(w, converter.MermaidJS)
	})
	mux.HandleFunc("/_mdtool/mathjax.min.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		fmt.Fprint(w, converter.MathJaxJS)
	})
	mux.HandleFunc("/events", s.handleEvents)
	mux.HandleFunc("/", s.handle)
	fmt.Printf("Starting server on %s serving %s\n", s.Listen, s.Dir)
	return http.ListenAndServe(s.Listen, mux)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	client := make(chan struct{})
	s.mu.Lock()
	s.clients = append(s.clients, client)
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		for i, c := range s.clients {
			if c == client {
				s.clients = append(s.clients[:i], s.clients[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
	}()

	notify := r.Context().Done()
	for {
		select {
		case <-client:
			fmt.Fprintf(w, "data: reload\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-notify:
			return
		}
	}
}

func (s *Server) notifyClients() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, client := range s.clients {
		select {
		case client <- struct{}{}:
		default:
		}
	}
}

func (s *Server) watchFiles() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Watcher error: %v", err)
		return
	}
	defer watcher.Close()

	filepath.Walk(s.Dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			watcher.Add(path)
		}
		return nil
	})

	var timer *time.Timer
	const delay = 100 * time.Millisecond

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					watcher.Add(event.Name)
				}
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				if strings.HasSuffix(event.Name, ".md") {
					if timer != nil {
						timer.Stop()
					}
					timer = time.AfterFunc(delay, func() {
						s.notifyClients()
					})
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	relPath := strings.TrimPrefix(r.URL.Path, "/")

	baseDir, err := filepath.Abs(s.Dir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fullPath := filepath.Clean(filepath.Join(baseDir, relPath))

	rel, err := filepath.Rel(baseDir, fullPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		http.NotFound(w, r)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if info.IsDir() {
		if !strings.HasSuffix(r.URL.Path, "/") {
			http.Redirect(w, r, r.URL.Path+"/", http.StatusFound)
			return
		}

		// Look for index.md or README.md
		for _, name := range []string{"index.md", "README.md"} {
			indexPath := filepath.Join(fullPath, name)
			if _, err := os.Stat(indexPath); err == nil {
				s.serveMarkdown(w, indexPath)
				return
			}
		}
		// Fallback to directory listing
		s.serveDirectoryListing(w, fullPath, r.URL.Path)
		return
	}

	if strings.HasSuffix(strings.ToLower(fullPath), ".md") {
		s.serveMarkdown(w, fullPath)
		return
	}

	if s.OnlyMD {
		http.NotFound(w, r)
		return
	}

	// Serve other files directly
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, fullPath)
}

func (s *Server) serveMarkdown(w http.ResponseWriter, path string) {
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	var buf bytes.Buffer
	if err := s.Converter.ConvertWithFilename(f, &buf, path); err != nil {
		http.Error(w, fmt.Sprintf("Conversion error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.Write(buf.Bytes())
}

func (s *Server) serveDirectoryListing(w http.ResponseWriter, fullPath, urlPath string) {
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var items []dirListingItem
	for _, entry := range entries {
		name := entry.Name()
		isDir := entry.IsDir()
		if isDir {
			name += "/"
		} else if s.OnlyMD && !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		items = append(items, dirListingItem{
			Name:  name,
			Href:  name,
			IsDir: isDir,
		})
	}

	data := dirListingData{
		URLPath:    urlPath,
		ShowParent: urlPath != "/",
		Items:      items,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if err := dirListingTemplate.Execute(w, data); err != nil {
		fmt.Fprintf(os.Stderr, "Template execution error: %v\n", err)
	}
}
