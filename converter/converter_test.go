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
