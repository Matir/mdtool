package converter

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// KindMermaidBlock is the NodeKind for Mermaid diagram blocks.
var KindMermaidBlock = ast.NewNodeKind("MermaidBlock")

// MermaidBlock represents a Mermaid diagram block in the AST.
type MermaidBlock struct {
	ast.BaseBlock
}

// Dump dumps the AST representation of the MermaidBlock.
func (b *MermaidBlock) Dump(source []byte, level int) {
	ast.DumpHelper(b, source, level, nil, nil)
}

// Kind returns KindMermaidBlock.
func (b *MermaidBlock) Kind() ast.NodeKind {
	return KindMermaidBlock
}

// NewMermaidBlock returns a new MermaidBlock.
func NewMermaidBlock() *MermaidBlock {
	return &MermaidBlock{
		BaseBlock: ast.BaseBlock{},
	}
}

type mermaidASTTransformer struct{}

func (t *mermaidASTTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()
	var toReplace [][2]ast.Node

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindFencedCodeBlock {
			fc := n.(*ast.FencedCodeBlock)
			lang := string(fc.Language(source))
			if strings.EqualFold(lang, "mermaid") {
				mb := NewMermaidBlock()
				mb.SetLines(fc.Lines())
				toReplace = append(toReplace, [2]ast.Node{fc, mb})
			}
		}
		return ast.WalkContinue, nil
	})

	for _, pair := range toReplace {
		oldNode, newNode := pair[0], pair[1]
		if parent := oldNode.Parent(); parent != nil {
			parent.ReplaceChild(parent, oldNode, newNode)
		}
	}
}

type mermaidHTMLRenderer struct {
	gmhtml.Config
}

func (r *mermaidHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindMermaidBlock, r.renderMermaidBlock)
}

func (r *mermaidHTMLRenderer) renderMermaidBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*MermaidBlock)
	if entering {
		_, _ = w.WriteString("<pre class=\"mermaid\">")
		l := n.Lines().Len()
		for i := 0; i < l; i++ {
			line := n.Lines().At(i)
			r.Writer.RawWrite(w, line.Value(source))
		}
	} else {
		_, _ = w.WriteString("</pre>\n")
	}
	return ast.WalkContinue, nil
}

type mermaidExtender struct{}

func (e *mermaidExtender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithASTTransformers(
			util.Prioritized(&mermaidASTTransformer{}, 100),
		),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(&mermaidHTMLRenderer{Config: gmhtml.NewConfig()}, 100),
		),
	)
}
