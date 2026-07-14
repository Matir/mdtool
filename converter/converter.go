package converter

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"io"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

const (
	FrontmatterAuto    = "auto"
	FrontmatterRemove  = "remove"
	FrontmatterInclude = "include"
)

//go:embed default.css
var defaultCSS string

//go:embed mermaid.min.js
var MermaidJS string

var pageTemplate = template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    {{- if .CSS }}
    <style>
    {{ .CSS }}
    </style>
    {{- end }}
</head>
<body>
    <div class="markdown-body">
    {{ .Content }}
    </div>
    {{- if .Mermaid }}
        {{- if .EmbedAssets }}
        <script>
        {{ .MermaidJS }}
        </script>
        {{- else }}
        <script src="/_mdtool/mermaid.min.js"></script>
        {{- end }}
        <script type="module">
        mermaid.initialize({ startOnLoad: true });
        </script>
    {{- end }}
    {{- if .Watch }}
    <script>
    const evtSource = new EventSource("/events");
    evtSource.onmessage = (event) => {
        if (event.data === "reload") {
            window.location.reload();
        }
    };
    </script>
    {{- end }}
</body>
</html>`))

// Converter holds configuration options for the conversion process.
type Converter struct {
	CSS         string
	Highlight   bool
	Mermaid     bool
	Watch       bool
	EmbedAssets bool
	Frontmatter string
	gm          goldmark.Markdown
}

// New returns a new Converter with the specified options.
func New(css string, highlight bool, mermaid bool) *Converter {
	if css == "" {
		css = defaultCSS
	}

	extensions := []goldmark.Extender{
		extension.GFM,
	}

	if highlight {
		extensions = append(extensions, highlighting.NewHighlighting())
	}

	gm := goldmark.New(
		goldmark.WithExtensions(extensions...),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)

	return &Converter{
		CSS:         css,
		Highlight:   highlight,
		Mermaid:     mermaid,
		EmbedAssets: true, // Default to true for batch mode
		Frontmatter: FrontmatterAuto,
		gm:          gm,
	}
}

// Convert renders Markdown from r to HTML in w.
func (c *Converter) Convert(r io.Reader, w io.Writer) error {
	md, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	md, err = processFrontmatter(md, c.Frontmatter)
	if err != nil {
		return err
	}

	// Use a pipe or buffer if we want to be more efficient,
	// but for template execution we need the rendered content.
	// For now, let's render to a string and use template.HTML to avoid escaping.
	var content bytes.Buffer
	if err := c.gm.Convert(md, &content); err != nil {
		return err
	}

	data := struct {
		CSS         template.CSS
		Content     template.HTML
		Mermaid     bool
		MermaidJS   template.JS
		EmbedAssets bool
		Watch       bool
	}{
		CSS:         template.CSS(c.CSS),
		Content:     template.HTML(content.String()),
		Mermaid:     c.Mermaid,
		MermaidJS:   template.JS(MermaidJS),
		EmbedAssets: c.EmbedAssets,
		Watch:       c.Watch,
	}

	return pageTemplate.Execute(w, data)
}

func processFrontmatter(src []byte, mode string) ([]byte, error) {
	if mode == "" {
		mode = FrontmatterAuto
	}

	switch mode {
	case FrontmatterAuto, FrontmatterRemove, FrontmatterInclude:
		// valid
	default:
		return nil, fmt.Errorf("invalid frontmatter mode: %q", mode)
	}

	if mode == FrontmatterInclude || len(src) == 0 {
		return src, nil
	}

	// Check if the first line is ---
	firstLineEnd := bytes.IndexByte(src, '\n')
	var firstLine []byte
	var restStart int
	if firstLineEnd == -1 {
		firstLine = src
		restStart = len(src)
	} else {
		firstLine = src[:firstLineEnd]
		restStart = firstLineEnd + 1
	}

	if string(bytes.TrimRight(firstLine, "\r")) != "---" {
		return src, nil
	}

	// First line is ---, now find the second line with ---
	curr := restStart
	secondLineStart := -1
	secondLineEnd := -1

	for curr < len(src) {
		lineEnd := bytes.IndexByte(src[curr:], '\n')
		var line []byte
		var nextCurr int
		if lineEnd == -1 {
			line = src[curr:]
			nextCurr = len(src)
		} else {
			line = src[curr : curr+lineEnd]
			nextCurr = curr + lineEnd + 1
		}

		if string(bytes.TrimRight(line, "\r")) == "---" {
			secondLineStart = curr
			secondLineEnd = nextCurr
			break
		}

		curr = nextCurr
	}

	if secondLineStart == -1 {
		// No second line with --- found
		return src, nil
	}

	frontmatterContent := src[restStart:secondLineStart]

	if mode == FrontmatterRemove {
		return src[secondLineEnd:], nil
	}

	// mode == FrontmatterAuto
	var node yaml.Node
	if err := yaml.Unmarshal(frontmatterContent, &node); err == nil {
		return src[secondLineEnd:], nil
	}

	return src, nil
}
