package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Matir/mdtool/converter"
)

func TestServerHandle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mdtool-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	if err := os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte("# Test File"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "other.txt"), []byte("not markdown"), 0644); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "index.md"), []byte("# Sub Index"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "foo&bar.md"), []byte("# Foo Bar"), 0644); err != nil {
		t.Fatal(err)
	}

	c := converter.New("", false, false)
	s := New(tmpDir, "", false, c)

	tests := []struct {
		name           string
		path           string
		onlyMD         bool
		wantStatus     int
		wantInBody     string
		dontWantInBody string
	}{
		{
			name:       "Serve Markdown file",
			path:       "/test.md",
			wantStatus: http.StatusOK,
			wantInBody: "<h1 id=\"test-file\">Test File</h1>",
		},
		{
			name:       "Serve index.md from directory",
			path:       "/sub/",
			wantStatus: http.StatusOK,
			wantInBody: "<h1 id=\"sub-index\">Sub Index</h1>",
		},
		{
			name:       "Serve non-markdown file when OnlyMD is false",
			path:       "/other.txt",
			onlyMD:     false,
			wantStatus: http.StatusOK,
			wantInBody: "not markdown",
		},
		{
			name:       "Don't serve non-markdown file when OnlyMD is true",
			path:       "/other.txt",
			onlyMD:     true,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Directory listing",
			path:       "/",
			wantStatus: http.StatusOK,
			wantInBody: "test.md",
		},
		{
			name:           "Directory listing with OnlyMD",
			path:           "/",
			onlyMD:         true,
			wantStatus:     http.StatusOK,
			wantInBody:     "test.md",
			dontWantInBody: "other.txt",
		},
		{
			name:           "Directory listing escapes HTML in filenames",
			path:           "/",
			wantStatus:     http.StatusOK,
			wantInBody:     "foo&amp;bar.md",
			dontWantInBody: "foo&bar.md",
		},
		{
			name:       "Path traversal prevention",
			path:       "/../secret.txt",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Redirect directory without trailing slash",
			path:       "/sub",
			wantStatus: http.StatusFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s.OnlyMD = tt.onlyMD
			req := httptest.NewRequest("GET", tt.path, nil)
			rr := httptest.NewRecorder()

			s.handle(rr, req)

			if status := rr.Code; status != tt.wantStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.wantStatus)
			}

			if status := rr.Code; status == http.StatusOK {
				if nosniff := rr.Header().Get("X-Content-Type-Options"); nosniff != "nosniff" {
					t.Errorf("expected X-Content-Type-Options: nosniff, got %q", nosniff)
				}
			}

			if tt.wantInBody != "" {
				body := rr.Body.String()
				if !strings.Contains(body, tt.wantInBody) {
					t.Errorf("handler body does not contain expected string %q", tt.wantInBody)
				}
			}

			if tt.dontWantInBody != "" {
				body := rr.Body.String()
				if strings.Contains(body, tt.dontWantInBody) {
					t.Errorf("handler body contains unwanted string %q", tt.dontWantInBody)
				}
			}
		})
	}
}
