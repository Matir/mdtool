package converter

import (
	"bytes"
	_ "embed"
	"fmt"
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

	MaxInputSize = 100 * 1024 * 1024 // 100 MB
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
    {{- if .Title }}
    <title>{{ .Title }}</title>
    {{- end }}
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
        mermaid.initialize({ startOnLoad: true, theme: 'dark' });
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
		extensions = append(extensions, highlighting.NewHighlighting(
			highlighting.WithStyle("dracula"),
		))
	}

	if mermaid {
		extensions = append(extensions, &mermaidExtender{})
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
	return c.ConvertWithFilename(r, w, "")
}

// ConvertWithFilename renders Markdown from r to HTML in w, using filename for title generation if needed.
func (c *Converter) ConvertWithFilename(r io.Reader, w io.Writer, filename string) error {
	md, err := io.ReadAll(io.LimitReader(r, MaxInputSize+1))
	if err != nil {
		return err
	}
	if int64(len(md)) > MaxInputSize {
		return fmt.Errorf("input exceeds maximum size of 100MB")
	}

	fmTitle := extractFrontmatterTitle(md)

	md, err = processFrontmatter(md, c.Frontmatter)
	if err != nil {
		return err
	}

	reader := text.NewReader(md)
	doc := c.gm.Parser().Parse(reader)

	var firstHeadingLevel int
	var firstH1Text string
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindHeading {
			heading := n.(*ast.Heading)
			if firstHeadingLevel == 0 {
				firstHeadingLevel = heading.Level
			}
			if heading.Level == 1 && firstH1Text == "" {
				firstH1Text = string(heading.Text(md))
			}
			if firstHeadingLevel > 0 && firstH1Text != "" {
				return ast.WalkStop, nil
			}
		}
		return ast.WalkContinue, nil
	})

	addTitleMode := strings.ToLower(c.AddTitle)
	if addTitleMode == "" {
		addTitleMode = AddTitleAuto
	}

	docTitle := fmTitle

	switch addTitleMode {
	case AddTitleOff, AddTitleFalse:
		// do nothing
	case AddTitleOn, AddTitleTrue, AddTitleAuto:
		shouldAdd := false
		if addTitleMode == AddTitleOn || addTitleMode == AddTitleTrue {
			shouldAdd = true
		} else {
			shouldAdd = (firstHeadingLevel != 1)
		}

		if shouldAdd {
			titleString := fmTitle
			if titleString == "" {
				if filename == "" {
					if f, ok := r.(*os.File); ok && f.Name() != "" && f.Name() != "/dev/stdin" {
						filename = f.Name()
					}
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

			docTitle = titleString

			escapedTitle := strings.NewReplacer("<", "&lt;", ">", "&gt;").Replace(titleString)
			md = append([]byte("# "+escapedTitle+"\n\n"), md...)
			doc = c.gm.Parser().Parse(text.NewReader(md))
		}
	default:
		return fmt.Errorf("invalid addtitle mode: %q", c.AddTitle)
	}

	if docTitle == "" {
		if firstH1Text != "" {
			docTitle = firstH1Text
		} else if filename != "" {
			base := filepath.Base(filename)
			ext := filepath.Ext(base)
			rawName := strings.TrimSuffix(base, ext)
			docTitle = toTitleCase(rawName)
		} else {
			docTitle = "Untitled"
		}
	}

	var content bytes.Buffer
	if err := c.gm.Renderer().Render(&content, md, doc); err != nil {
		return err
	}

	data := struct {
		Title       string
		CSS         template.CSS
		Content     template.HTML
		Mermaid     bool
		MermaidJS   template.JS
		EmbedAssets bool
		Watch       bool
	}{
		Title:       docTitle,
		CSS:         template.CSS(c.CSS),
		Content:     template.HTML(content.String()),
		Mermaid:     c.Mermaid,
		MermaidJS:   template.JS(MermaidJS),
		EmbedAssets: c.EmbedAssets,
		Watch:       c.Watch,
	}

	return pageTemplate.Execute(w, data)
}

func parseFrontmatterBounds(src []byte) (fmContent []byte, restStart int, secondLineEnd int, found bool) {
	if len(src) == 0 {
		return nil, 0, 0, false
	}

	firstLineEnd := bytes.IndexByte(src, '\n')
	var firstLine []byte
	if firstLineEnd == -1 {
		firstLine = src
		restStart = len(src)
	} else {
		firstLine = src[:firstLineEnd]
		restStart = firstLineEnd + 1
	}

	if string(bytes.TrimRight(firstLine, " \t\r")) != "---" {
		return nil, 0, 0, false
	}

	curr := restStart
	secondLineStart := -1
	secondLineEnd = -1

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

		if string(bytes.TrimRight(line, " \t\r")) == "---" {
			secondLineStart = curr
			secondLineEnd = nextCurr
			break
		}

		curr = nextCurr
	}

	if secondLineStart == -1 {
		return nil, 0, 0, false
	}

	return src[restStart:secondLineStart], restStart, secondLineEnd, true
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

	fmContent, _, secondLineEnd, ok := parseFrontmatterBounds(src)
	if !ok {
		return src, nil
	}

	if mode == FrontmatterRemove {
		return src[secondLineEnd:], nil
	}

	// mode == FrontmatterAuto
	if isYAMLMapping(fmContent) {
		return src[secondLineEnd:], nil
	}

	return src, nil
}

func isYAMLMapping(data []byte) bool {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return false
	}
	if node.Kind == yaml.MappingNode {
		return true
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 && node.Content[0].Kind == yaml.MappingNode {
		return true
	}
	return false
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
	fmContent, _, _, ok := parseFrontmatterBounds(src)
	if !ok {
		return ""
	}

	var node map[string]any
	if err := yaml.Unmarshal(fmContent, &node); err == nil && node != nil {
		if val, ok := node["title"]; ok && val != nil {
			titleStr := fmt.Sprintf("%v", val)
			return strings.TrimSpace(titleStr)
		}
	}

	return ""
}
