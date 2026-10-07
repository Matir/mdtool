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
	c := New("", true, true)
	input := "```mermaid\ngraph TD;\n    A-->B;\n    B-->C[Value < 10 & > 0];\n```\n\n```go\nfunc main() {}\n```"
	var buf bytes.Buffer
	if err := c.Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "mermaid.initialize") {
		t.Errorf("Expected mermaid script in output, got %s", output)
	}
	if !strings.Contains(output, "<pre class=\"mermaid\">graph TD;\n    A--&gt;B;\n    B--&gt;C[Value &lt; 10 &amp; &gt; 0];\n</pre>") {
		t.Errorf("Expected <pre class=\"mermaid\"> block with escaped content in output, got:\n%s", output)
	}
	if strings.Contains(output, "language-mermaid") {
		t.Errorf("Did not expect language-mermaid code block when mermaid is enabled, got:\n%s", output)
	}
	// Verify syntax highlighting still works for other languages
	if !strings.Contains(output, "color:") && !strings.Contains(output, "style=") {
		t.Errorf("Expected syntax highlighting for Go block in output, got:\n%s", output)
	}
}

func TestConvertMermaidDisabled(t *testing.T) {
	c := New("", false, false)
	input := "```mermaid\ngraph TD;\n    A-->B;\n```"
	var buf bytes.Buffer
	if err := c.Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "mermaid.initialize") {
		t.Errorf("Did not expect mermaid script when mermaid is disabled")
	}
	if strings.Contains(output, "<pre class=\"mermaid\">") {
		t.Errorf("Did not expect <pre class=\"mermaid\"> when mermaid is disabled")
	}
	if !strings.Contains(output, "<pre><code class=\"language-mermaid\">") {
		t.Errorf("Expected standard code block <pre><code class=\"language-mermaid\"> when mermaid is disabled, got:\n%s", output)
	}
}

func TestConvertMermaidCaseInsensitive(t *testing.T) {
	c := New("", false, true)
	input := "```Mermaid\ngraph LR;\n    X-->Y;\n```"
	var buf bytes.Buffer
	if err := c.Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "<pre class=\"mermaid\">graph LR;\n    X--&gt;Y;\n</pre>") {
		t.Errorf("Expected <pre class=\"mermaid\"> for case-insensitive Mermaid fence, got:\n%s", output)
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

func TestConvertMathJaxInline(t *testing.T) {
	c := New("", false, false)
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "basic inline math",
			input: "Inline math $1+2$ here.",
			want:  `<p>Inline math <span class="math inline">\(1+2\)</span> here.</p>`,
		},
		{
			name:  "latex fraction and subscripts",
			input: `Equation $\frac{a_1}{b_2}$ with subscripts.`,
			want:  `<p>Equation <span class="math inline">\(\frac{a_1}{b_2}\)</span> with subscripts.</p>`,
		},
		{
			name:  "neighboring inline math",
			input: "$a$+$b$ and $c$, $d$",
			want:  `<p><span class="math inline">\(a\)</span>+<span class="math inline">\(b\)</span> and <span class="math inline">\(c\)</span>, <span class="math inline">\(d\)</span></p>`,
		},
		{
			name:  "parenthesized inline math",
			input: "See ($x + y$) for details.",
			want:  `<p>See (<span class="math inline">\(x + y\)</span>) for details.</p>`,
		},
		{
			name:  "escaped dollar inside inline math",
			input: `Formula $x = \$5$ with literal dollar.`,
			want:  `<p>Formula <span class="math inline">\(x = \$5\)</span> with literal dollar.</p>`,
		},
		{
			name:  "html special characters in inline math",
			input: "$a < b & c > d$",
			want:  `<p><span class="math inline">\(a &lt; b &amp; c &gt; d\)</span></p>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := c.Convert(strings.NewReader(tt.input), &buf); err != nil {
				t.Fatalf("Convert failed: %v", err)
			}
			output := buf.String()
			if !strings.Contains(output, tt.want) {
				t.Errorf("Expected %q in output, got:\n%s", tt.want, output)
			}
			if !strings.Contains(output, "window.MathJax") {
				t.Errorf("Expected window.MathJax configuration in output")
			}
		})
	}
}

func TestConvertMathJaxBlock(t *testing.T) {
	c := New("", false, false)
	input := "$$\n\\mathbb{E}(X) = \\int x dF(x)\n$$\n$$\na < b & c > d\n$$\n\n$$ E = mc^2 $$\n\n```text\n$not_math$\n$$not_block$$\n```\n\n`$also_not_math$`\n"
	var buf bytes.Buffer
	if err := c.Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output := buf.String()
	wantBlocks := []string{
		"<p><span class=\"math display\">\\[\\mathbb{E}(X) = \\int x dF(x)\n\\]</span></p>",
		"<p><span class=\"math display\">\\[a &lt; b &amp; c &gt; d\n\\]</span></p>",
		"<p><span class=\"math display\">\\[ E = mc^2 \\]</span></p>",
		"<code>$also_not_math$</code>",
	}
	for _, want := range wantBlocks {
		if !strings.Contains(output, want) {
			t.Errorf("Expected %q in output, got:\n%s", want, output)
		}
	}
	if strings.Contains(output, `\(not_math\)`) || strings.Contains(output, `\(also_not_math\)`) {
		t.Errorf("Code block or code span was unexpectedly parsed as math:\n%s", output)
	}
}

func TestConvertMathJaxCurrency(t *testing.T) {
	c := New("", false, false)
	tests := []struct {
		name           string
		input          string
		wantInBody     string
		dontWantInBody string
	}{
		{
			name:           "single currency symbol",
			input:          "The item costs $5.",
			wantInBody:     "<p>The item costs $5.</p>",
			dontWantInBody: `class="math`,
		},
		{
			name:           "two currency amounts in one sentence",
			input:          "It costs $5 and $10.",
			wantInBody:     "<p>It costs $5 and $10.</p>",
			dontWantInBody: `class="math`,
		},
		{
			name:           "currency amounts with commas and decimals",
			input:          "The total is $20,000.50 and $30,000.75.",
			wantInBody:     "<p>The total is $20,000.50 and $30,000.75.</p>",
			dontWantInBody: `class="math`,
		},
		{
			name:           "currency price ranges",
			input:          "Prices range from $5-$10 or $5/$10 or $5-$.50.",
			wantInBody:     "<p>Prices range from $5-$10 or $5/$10 or $5-$.50.</p>",
			dontWantInBody: `class="math`,
		},
		{
			name:           "parenthesized currency amounts",
			input:          "Tickets are ($5) or ($10), or $5 ($10 with tax).",
			wantInBody:     "<p>Tickets are ($5) or ($10), or $5 ($10 with tax).</p>",
			dontWantInBody: `class="math`,
		},
		{
			name:           "currency followed by parenthesized math",
			input:          "It costs $5 ($x$ with tax).",
			wantInBody:     `<p>It costs $5 (<span class="math inline">\(x\)</span> with tax).</p>`,
			dontWantInBody: `\(5`,
		},
		{
			name:           "currency interleaved with inline math",
			input:          "Cost is $5 for $a$ and $10 for $b$.",
			wantInBody:     `<p>Cost is $5 for <span class="math inline">\(a\)</span> and $10 for <span class="math inline">\(b\)</span>.</p>`,
			dontWantInBody: `\(5`,
		},
		{
			name:           "currency on line before math at start of next line",
			input:          "The price is $50 and the formula is\n$x + y$.",
			wantInBody:     "<p>The price is $50 and the formula is\n<span class=\"math inline\">\\(x + y\\)</span>.</p>",
			dontWantInBody: `\(50`,
		},
		{
			name:           "spaces immediately inside single dollar delimiters",
			input:          "This $ 5 + 5 $ has spaces.",
			wantInBody:     "<p>This $ 5 + 5 $ has spaces.</p>",
			dontWantInBody: `class="math`,
		},
		{
			name:           "trailing currency symbols after digits",
			input:          "In some places it is 100$ and 200$.",
			wantInBody:     "<p>In some places it is 100$ and 200$.</p>",
			dontWantInBody: `class="math`,
		},
		{
			name:           "word-internal dollar signs",
			input:          "Identifier foo$bar$baz is not math.",
			wantInBody:     "<p>Identifier foo$bar$baz is not math.</p>",
			dontWantInBody: `class="math`,
		},
		{
			name:           "escaped dollar signs",
			input:          `Escaped \$5 and \$10 are literal.`,
			wantInBody:     "<p>Escaped $5 and $10 are literal.</p>",
			dontWantInBody: `class="math`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := c.Convert(strings.NewReader(tt.input), &buf); err != nil {
				t.Fatalf("Convert failed: %v", err)
			}
			// Extract markdown-body to avoid matching anything inside embedded JS
			output := buf.String()
			bodyStart := strings.Index(output, `<div class="markdown-body">`)
			bodyEnd := strings.Index(output, `</div>`)
			if bodyStart == -1 || bodyEnd == -1 || bodyEnd < bodyStart {
				t.Fatalf("Could not locate markdown-body in output")
			}
			body := output[bodyStart:bodyEnd]

			if !strings.Contains(body, tt.wantInBody) {
				t.Errorf("Expected body to contain %q, got:\n%s", tt.wantInBody, body)
			}
			if tt.dontWantInBody != "" && strings.Contains(body, tt.dontWantInBody) {
				t.Errorf("Expected body to NOT contain %q, got:\n%s", tt.dontWantInBody, body)
			}
		})
	}
}

func TestConvertMathJaxDisabled(t *testing.T) {
	c := New("", false, false)
	c.MathJax = false
	input := "Inline $1+2$ and block:\n\n$$\n1+2\n$$"
	var buf bytes.Buffer
	if err := c.Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "window.MathJax") || strings.Contains(output, "mathjax.min.js") {
		t.Errorf("Did not expect MathJax script when MathJax is disabled")
	}
	if strings.Contains(output, `class="math`) {
		t.Errorf("Did not expect math spans when MathJax is disabled, got:\n%s", output)
	}
	if !strings.Contains(output, "<p>Inline $1+2$ and block:</p>") {
		t.Errorf("Expected literal $1+2$ when MathJax is disabled, got:\n%s", output)
	}
}

func TestConvertMathJaxServerModeScript(t *testing.T) {
	c := New("", false, false)
	c.EmbedAssets = false
	input := "$x^2$"
	var buf bytes.Buffer
	if err := c.Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("Convert failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `<script src="/_mdtool/mathjax.min.js"></script>`) {
		t.Errorf("Expected external mathjax script tag when EmbedAssets is false, got:\n%s", output)
	}
}
