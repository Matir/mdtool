package converter

import (
	"bytes"
	"strings"
	"testing"
)

func TestConvertBasic(t *testing.T) {
	c := New("", false, false)
	input := "# Hello World"
	var buf bytes.Buffer
	if err := c.Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "<h1 id=\"hello-world\">Hello World</h1>") {
		t.Errorf("Expected <h1 id=\"hello-world\">Hello World</h1> in output, got %s", output)
	}
	if !strings.Contains(output, "<style>") {
		t.Errorf("Expected <style> tag in output for default CSS")
	}
}

func TestConvertGFM(t *testing.T) {
	c := New("", false, false)
	input := "| header |\n| --- |\n| cell |"
	var buf bytes.Buffer
	if err := c.Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "<table>") {
		t.Errorf("Expected <table> in output for GFM, got %s", output)
	}
}

func TestConvertHighlight(t *testing.T) {
	c := New("", true, false)
	input := "```go\nfunc main() {}\n```"
	var buf bytes.Buffer
	if err := c.Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output := buf.String()
	// Chroma usually adds inline styles or specific classes
	if !strings.Contains(output, "color:") && !strings.Contains(output, "style=") {
		t.Errorf("Expected syntax highlighting in output, got %s", output)
	}
}

func TestConvertMermaid(t *testing.T) {
	c := New("", false, true)
	input := "graph TD; A-->B;"
	var buf bytes.Buffer
	if err := c.Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "mermaid.initialize") {
		t.Errorf("Expected mermaid script in output, got %s", output)
	}
}

func TestConvertCustomCSS(t *testing.T) {
	customCSS := "body { color: red; }"
	c := New(customCSS, false, false)
	input := "# Red Header"
	var buf bytes.Buffer
	if err := c.Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, customCSS) {
		t.Errorf("Expected custom CSS in output, got %s", output)
	}
}

func TestProcessFrontmatter(t *testing.T) {
	validYAML := "---\ntitle: Test Document\nauthor: Alice\n---\n# Title\nBody content"
	invalidYAML := "---\ntitle: : invalid yaml syntax {\n---\n# Title\nBody content"
	noSecondDelimiter := "---\ntitle: Test Document\n# Title\nBody content"
	noFirstDelimiter := "# Title\n---\ntitle: Test Document\n---\nBody content"

	tests := []struct {
		name     string
		mode     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "auto with valid YAML",
			mode:     FrontmatterAuto,
			input:    validYAML,
			expected: "# Title\nBody content",
		},
		{
			name:     "auto with invalid YAML",
			mode:     FrontmatterAuto,
			input:    invalidYAML,
			expected: invalidYAML,
		},
		{
			name:     "auto with no second delimiter",
			mode:     FrontmatterAuto,
			input:    noSecondDelimiter,
			expected: noSecondDelimiter,
		},
		{
			name:     "auto with no first delimiter",
			mode:     FrontmatterAuto,
			input:    noFirstDelimiter,
			expected: noFirstDelimiter,
		},
		{
			name:     "remove with valid YAML",
			mode:     FrontmatterRemove,
			input:    validYAML,
			expected: "# Title\nBody content",
		},
		{
			name:     "remove with invalid YAML",
			mode:     FrontmatterRemove,
			input:    invalidYAML,
			expected: "# Title\nBody content",
		},
		{
			name:     "remove with no second delimiter",
			mode:     FrontmatterRemove,
			input:    noSecondDelimiter,
			expected: noSecondDelimiter,
		},
		{
			name:     "include with valid YAML",
			mode:     FrontmatterInclude,
			input:    validYAML,
			expected: validYAML,
		},
		{
			name:     "auto with scalar text inside delimiters",
			mode:     FrontmatterAuto,
			input:    "---\nJust plain text\n---\n# Body",
			expected: "---\nJust plain text\n---\n# Body",
		},
		{
			name:     "auto with array inside delimiters",
			mode:     FrontmatterAuto,
			input:    "---\n- item 1\n- item 2\n---\n# Body",
			expected: "---\n- item 1\n- item 2\n---\n# Body",
		},
		{
			name:    "invalid frontmatter mode",
			mode:    "invalid",
			input:   validYAML,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := processFrontmatter([]byte(tt.input), tt.mode)
			if (err != nil) != tt.wantErr {
				t.Fatalf("processFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && string(got) != tt.expected {
				t.Errorf("processFrontmatter() = %q, want %q", string(got), tt.expected)
			}
		})
	}
}

func TestToTitleCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-first_post", "My First Post"},
		{"hello world", "Hello World"},
		{"sample_file_name.md", "Sample File Name.md"},
		{"api-v2-spec", "Api V2 Spec"},
	}

	for _, tt := range tests {
		got := toTitleCase(tt.input)
		if got != tt.expected {
			t.Errorf("toTitleCase(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestAddTitle(t *testing.T) {
	tests := []struct {
		name         string
		addTitleMode string
		filename     string
		input        string
		wantHeading  string
		dontWant     string
		wantErr      bool
	}{
		{
			name:         "auto mode with existing h1",
			addTitleMode: AddTitleAuto,
			filename:     "test-doc.md",
			input:        "# Existing H1\n\nSome body",
			dontWant:     "<h1 id=\"test-doc\">Test Doc</h1>",
		},
		{
			name:         "auto mode with h2 first",
			addTitleMode: AddTitleAuto,
			filename:     "test-doc.md",
			input:        "## Subheading\n\nSome body",
			wantHeading:  "<h1 id=\"test-doc\">Test Doc</h1>",
		},
		{
			name:         "auto mode with frontmatter title",
			addTitleMode: AddTitleAuto,
			filename:     "test-doc.md",
			input:        "---\ntitle: Custom Frontmatter Title\n---\n\nSome body",
			wantHeading:  "<h1 id=\"custom-frontmatter-title\">Custom Frontmatter Title</h1>",
		},
		{
			name:         "auto mode with HTML in frontmatter title",
			addTitleMode: AddTitleAuto,
			filename:     "test-doc.md",
			input:        "---\ntitle: \"<script>alert(1)</script> Escaped\"\n---\n\nBody",
			wantHeading:  "&lt;script&gt;alert(1)&lt;/script&gt; Escaped",
		},
		{
			name:         "on mode with existing h1",
			addTitleMode: AddTitleOn,
			filename:     "test-doc.md",
			input:        "# Existing H1\n\nSome body",
			wantHeading:  "<h1 id=\"test-doc\">Test Doc</h1>",
		},
		{
			name:         "off mode with no h1",
			addTitleMode: AddTitleOff,
			filename:     "test-doc.md",
			input:        "Some body without heading",
			dontWant:     "<h1",
		},
		{
			name:         "invalid addtitle mode",
			addTitleMode: "invalid-mode",
			input:        "Some content",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New("", false, false)
			c.AddTitle = tt.addTitleMode

			var buf bytes.Buffer
			err := c.ConvertWithFilename(strings.NewReader(tt.input), &buf, tt.filename)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Convert() error = %v, wantErr %v", err, tt.wantErr)
			}

			output := buf.String()
			if tt.wantHeading != "" && !strings.Contains(output, tt.wantHeading) {
				t.Errorf("Expected output to contain %q, got:\n%s", tt.wantHeading, output)
			}
			if tt.dontWant != "" && strings.Contains(output, tt.dontWant) {
				t.Errorf("Expected output to NOT contain %q, got:\n%s", tt.dontWant, output)
			}
		})
	}
}

func TestFrontmatterTrailingSpaces(t *testing.T) {
	inputWithTrailingSpaces := "--- \t\r\ntitle: Space Delimiter\n---  \r\n# Body"
	got, err := processFrontmatter([]byte(inputWithTrailingSpaces), FrontmatterAuto)
	if err != nil {
		t.Fatalf("processFrontmatter error: %v", err)
	}
	if strings.Contains(string(got), "title: Space Delimiter") {
		t.Errorf("Expected frontmatter with trailing spaces on delimiter to be removed, got:\n%s", string(got))
	}

	invalidLeadingSpaces := "  ---\ntitle: Invalid Delimiter\n---\n# Body"
	gotLeading, err := processFrontmatter([]byte(invalidLeadingSpaces), FrontmatterAuto)
	if err != nil {
		t.Fatalf("processFrontmatter error: %v", err)
	}
	if !strings.Contains(string(gotLeading), "title: Invalid Delimiter") {
		t.Errorf("Expected frontmatter with leading spaces on delimiter to NOT be removed, got:\n%s", string(gotLeading))
	}
}

func TestAddTitleEscapingAmpersand(t *testing.T) {
	c := New("", false, false)
	c.AddTitle = AddTitleAuto
	input := "---\ntitle: \"Fish & Chips & <script>alert(1)</script>\"\n---\n\nBody content"
	var buf bytes.Buffer
	if err := c.Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "&amp;amp;") {
		t.Errorf("Title ampersand was double-escaped: %s", output)
	}
	if !strings.Contains(output, "Fish &amp; Chips") {
		t.Errorf("Expected 'Fish &amp; Chips' in title output, got:\n%s", output)
	}
	if !strings.Contains(output, "&lt;script&gt;") {
		t.Errorf("Expected script tag to be escaped to &lt;script&gt;, got:\n%s", output)
	}
}

type oversizedReader struct {
	remaining int64
}

func (r *oversizedReader) Read(p []byte) (n int, err error) {
	if r.remaining <= 0 {
		return 0, nil
	}
	toRead := int64(len(p))
	if toRead > r.remaining {
		toRead = r.remaining
	}
	for i := int64(0); i < toRead; i++ {
		p[i] = 'a'
	}
	r.remaining -= toRead
	return int(toRead), nil
}

func TestConvertExceedsMaxInputSize(t *testing.T) {
	c := New("", false, false)
	r := &oversizedReader{remaining: MaxInputSize + 1024}
	var buf bytes.Buffer
	err := c.Convert(r, &buf)
	if err == nil {
		t.Fatal("Expected error for oversized input, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size of 100MB") {
		t.Errorf("Expected size limit error message, got: %v", err)
	}
}

func TestHTMLDocumentTitleTag(t *testing.T) {
	c := New("", false, false)
	input := "---\ntitle: Document Title Test\n---\n# Heading"
	var buf bytes.Buffer
	if err := c.Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "<title>Document Title Test</title>") {
		t.Errorf("Expected <title>Document Title Test</title> in head, got:\n%s", output)
	}
}
