package converter

import (
	"bytes"
	_ "embed"
	"fmt"
	"html"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

const (
	FrontmatterAuto    = "auto"
	FrontmatterRemove  = "remove"
	FrontmatterInclude = "include"

	AddTitleOff   = "off"
	AddTitleFalse = "false"
	AddTitleAuto  = "auto"
	AddTitleOn    = "on"
	AddTitleTrue  = "true"
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
	AddTitle    string
	Filename    string
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
			gmhtml.WithUnsafe(),
		),
	)

	return &Converter{
		CSS:         css,
		Highlight:   highlight,
		Mermaid:     mermaid,
		EmbedAssets: true, // Default to true for batch mode
		Frontmatter: FrontmatterAuto,
		AddTitle:    AddTitleAuto,
		gm:          gm,
	}
}

// Convert renders Markdown from r to HTML in w.
func (c *Converter) Convert(r io.Reader, w io.Writer) error {
	md, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	fmTitle := extractFrontmatterTitle(md)

	md, err = processFrontmatter(md, c.Frontmatter)
	if err != nil {
		return err
	}

	addTitleMode := strings.ToLower(c.AddTitle)
	if addTitleMode == "" {
		addTitleMode = AddTitleAuto
	}

	switch addTitleMode {
	case AddTitleOff, AddTitleFalse:
		// do nothing
	case AddTitleOn, AddTitleTrue, AddTitleAuto:
		shouldAdd := false
		if addTitleMode == AddTitleOn || addTitleMode == AddTitleTrue {
			shouldAdd = true
		} else {
			firstLevel := c.findFirstHeadingLevel(md)
			shouldAdd = (firstLevel != 1)
		}

		if shouldAdd {
			titleString := fmTitle
			if titleString == "" {
				var filename string
				if c.Filename != "" {
					filename = c.Filename
				} else if f, ok := r.(*os.File); ok && f.Name() != "" && f.Name() != "/dev/stdin" {
					filename = f.Name()
				}

				if filename != "" {
					base := filepath.Base(filename)
					ext := filepath.Ext(base)
					rawName := strings.TrimSuffix(base, ext)
					titleString = toTitleCase(rawName)
				} else {
					titleString = "Untitled"
				}
			}

			escapedTitle := html.EscapeString(titleString)
			md = append([]byte("# "+escapedTitle+"\n\n"), md...)
		}
	default:
		return fmt.Errorf("invalid addtitle mode: %q", c.AddTitle)
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

func toTitleCase(s string) string {
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)
	for i, w := range words {
		r := []rune(w)
		if len(r) > 0 {
			r[0] = unicode.ToUpper(r[0])
			words[i] = string(r)
		}
	}
	return strings.Join(words, " ")
}

func extractFrontmatterTitle(src []byte) string {
	if len(src) == 0 {
		return ""
	}

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
		return ""
	}

	curr := restStart
	secondLineStart := -1

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
			break
		}

		curr = nextCurr
	}

	if secondLineStart == -1 {
		return ""
	}

	frontmatterContent := src[restStart:secondLineStart]

	var node map[string]any
	if err := yaml.Unmarshal(frontmatterContent, &node); err == nil && node != nil {
		if val, ok := node["title"]; ok && val != nil {
			titleStr := fmt.Sprintf("%v", val)
			return strings.TrimSpace(titleStr)
		}
	}

	return ""
}

func (c *Converter) findFirstHeadingLevel(md []byte) int {
	reader := text.NewReader(md)
	doc := c.gm.Parser().Parse(reader)
	firstLevel := 0
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindHeading {
			heading := n.(*ast.Heading)
			firstLevel = heading.Level
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return firstLevel
}
