package converter

import (
	"bytes"

	mathjax "github.com/litao91/goldmark-mathjax"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type mathBlockData struct {
	indent     int
	fenceLen   int
	node       ast.Node
	singleLine bool
}

var mathBlockInfoKey = parser.NewContextKey()

type mathBlockParser struct{}

func (b *mathBlockParser) Trigger() []byte {
	return []byte{'$'}
}

func (b *mathBlockParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, segment := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 || pos >= len(line) || line[pos] != '$' {
		return nil, parser.NoChildren
	}

	i := pos
	for ; i < len(line) && line[i] == '$'; i++ {
	}
	fenceLen := i - pos
	if fenceLen < 2 {
		return nil, parser.NoChildren
	}

	// Case 1: Multi-line math block where the opening line has only "$$" and optional whitespace.
	if util.IsBlank(line[i:]) {
		node := mathjax.NewMathBlock()
		pc.Set(mathBlockInfoKey, &mathBlockData{
			indent:     pc.BlockIndent(),
			fenceLen:   fenceLen,
			node:       node,
			singleLine: false,
		})
		return node, parser.NoChildren
	}

	// Case 2: Single-line math block "$$ ... $$" on its own line.
	end := len(line)
	for end > i && isMathSpace(line[end-1]) {
		end--
	}
	closeStart := end
	for closeStart > i && line[closeStart-1] == '$' {
		closeStart--
	}
	if end-closeStart == fenceLen && closeStart > i && !escapedDollar(line, closeStart) {
		// Ensure there are no unescaped closing delimiters in the middle of the line.
		hasInnerFence := false
		for j := i; j < closeStart; j++ {
			if line[j] == '$' && !escapedDollar(line, j) {
				k := j
				for ; k < closeStart && line[k] == '$'; k++ {
				}
				if k-j >= fenceLen {
					hasInnerFence = true
					break
				}
				j = k - 1
			}
		}
		if !hasInnerFence {
			node := mathjax.NewMathBlock()
			contentStart := segment.Start - segment.Padding + i
			contentStop := segment.Start - segment.Padding + closeStart
			node.Lines().Append(text.NewSegment(contentStart, contentStop))
			reader.AdvanceToEOL()
			pc.Set(mathBlockInfoKey, &mathBlockData{
				indent:     pc.BlockIndent(),
				fenceLen:   fenceLen,
				node:       node,
				singleLine: true,
			})
			return node, parser.NoChildren
		}
	}

	return nil, parser.NoChildren
}

func (b *mathBlockParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	data, ok := pc.Get(mathBlockInfoKey).(*mathBlockData)
	if !ok || data == nil || data.node != node || data.singleLine {
		return parser.Close
	}

	line, segment := reader.PeekLine()
	w, pos := util.IndentWidth(line, reader.LineOffset())
	if w < 4 && pos < len(line) {
		i := pos
		for ; i < len(line) && line[i] == '$'; i++ {
		}
		length := i - pos
		if length >= data.fenceLen && util.IsBlank(line[i:]) {
			reader.AdvanceToEOL()
			return parser.Close
		}
	}

	pos, padding := util.IndentPositionPadding(line, reader.LineOffset(), segment.Padding, data.indent)
	if pos < 0 {
		pos = max(0, util.FirstNonSpacePosition(line)) - segment.Padding
		padding = 0
	}
	seg := text.NewSegmentPadding(segment.Start+pos, segment.Stop, padding)
	seg.ForceNewline = true
	node.Lines().Append(seg)
	reader.AdvanceToEOL()
	return parser.Continue | parser.NoChildren
}

func (b *mathBlockParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	data, ok := pc.Get(mathBlockInfoKey).(*mathBlockData)
	if ok && data != nil && data.node == node {
		pc.Set(mathBlockInfoKey, nil)
	}
}

func (b *mathBlockParser) CanInterruptParagraph() bool {
	return true
}

func (b *mathBlockParser) CanAcceptIndentedLine() bool {
	return false
}

type inlineMathParser struct{}

func (s *inlineMathParser) Trigger() []byte {
	return []byte{'$'}
}

func (s *inlineMathParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, startSegment := block.PeekLine()
	opener := 0
	for ; opener < len(line) && line[opener] == '$'; opener++ {
	}
	if opener == 0 {
		return nil
	}

	source := block.Source()

	// More than 2 '$' is not a standard math delimiter.
	if opener > 2 {
		block.Advance(opener)
		return ast.NewTextSegment(startSegment.WithStop(startSegment.Start + opener))
	}

	// Pandoc / GFM rules for single '$' inline math to avoid currency false positives:
	// 1. Opening '$' must not be immediately preceded by an alphanumeric character (e.g. "100$" or "foo$bar$").
	// 2. Opening '$' must not be immediately followed by whitespace or end of line (e.g. "$ 5 + 5 $" or "$10").
	if opener == 1 {
		if startSegment.Start > 0 && isASCIIAlnum(source[startSegment.Start-1]) {
			block.Advance(opener)
			return ast.NewTextSegment(startSegment.WithStop(startSegment.Start + opener))
		}
		if opener >= len(line) || isMathSpace(line[opener]) {
			block.Advance(opener)
			return ast.NewTextSegment(startSegment.WithStop(startSegment.Start + opener))
		}
	}

	block.Advance(opener)
	l, pos := block.Position()
	node := mathjax.NewInlineMath()

	for {
		line, segment := block.PeekLine()
		if line == nil {
			block.SetPosition(l, pos)
			return ast.NewTextSegment(startSegment.WithStop(startSegment.Start + opener))
		}
		for i := 0; i < len(line); i++ {
			c := line[i]
			if c == '$' && !escapedDollar(line, i) {
				oldi := i
				for ; i < len(line) && line[i] == '$'; i++ {
				}
				closure := i - oldi
				if opener == 1 {
					if closure != 1 || !isValidSingleMathCloser(line, oldi, i) {
						block.SetPosition(l, pos)
						return ast.NewTextSegment(startSegment.WithStop(startSegment.Start + opener))
					}
					seg := segment.WithStop(segment.Start + oldi)
					if !seg.IsEmpty() {
						node.AppendChild(node, ast.NewRawTextSegment(seg))
					}
					if node.IsBlank(source) {
						block.SetPosition(l, pos)
						return ast.NewTextSegment(startSegment.WithStop(startSegment.Start + opener))
					}
					block.Advance(i)
					return node
				}
				if closure == opener {
					seg := segment.WithStop(segment.Start + oldi)
					if !seg.IsEmpty() {
						node.AppendChild(node, ast.NewRawTextSegment(seg))
					}
					block.Advance(i)
					goto end
				}
			}
		}
		if !util.IsBlank(line) {
			node.AppendChild(node, ast.NewRawTextSegment(segment))
		}
		block.AdvanceLine()
	}

end:
	if node.IsBlank(source) {
		block.SetPosition(l, pos)
		return ast.NewTextSegment(startSegment.WithStop(startSegment.Start + opener))
	}

	firstSeg := node.FirstChild().(*ast.Text).Segment
	lastSeg := node.LastChild().(*ast.Text).Segment
	if !firstSeg.IsEmpty() && source[firstSeg.Start] == ' ' &&
		!lastSeg.IsEmpty() && source[lastSeg.Stop-1] == ' ' {
		firstText := node.FirstChild().(*ast.Text)
		firstText.Segment = firstText.Segment.WithStart(firstText.Segment.Start + 1)
		lastText := node.LastChild().(*ast.Text)
		lastText.Segment = lastText.Segment.WithStop(lastText.Segment.Stop - 1)
	}
	return node
}

// isValidSingleMathCloser checks whether a single '$' at line[oldi] (with after = oldi+1)
// is a valid closing delimiter under Pandoc/GFM rules:
// - Must be immediately preceded by a non-space character (and not an opening bracket/paren like '(', '[', '{').
// - Must not be immediately followed by an alphanumeric character (especially a digit) or '.<digit>'.
func isValidSingleMathCloser(line []byte, oldi, after int) bool {
	if oldi <= 0 {
		return false
	}
	left := line[oldi-1]
	if isMathSpace(left) || left == '(' || left == '[' || left == '{' {
		return false
	}
	if after < len(line) {
		right := line[after]
		if isASCIIAlnum(right) {
			return false
		}
		if right == '.' && after+1 < len(line) && isASCIIDigit(line[after+1]) {
			return false
		}
	}
	return true
}

func escapedDollar(line []byte, offset int) bool {
	backslashes := 0
	for i := offset - 1; i >= 0 && line[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 != 0
}

func isMathSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func isASCIIDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isASCIIAlnum(c byte) bool {
	return isASCIIDigit(c) || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

type mathHTMLRenderer struct {
	gmhtml.Config
}

func (r *mathHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(mathjax.KindMathBlock, r.renderMathBlock)
	reg.Register(mathjax.KindInlineMath, r.renderInlineMath)
}

func (r *mathHTMLRenderer) renderMathBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*mathjax.MathBlock)
	if entering {
		_, _ = w.WriteString(`<p><span class="math display">\[`)
		l := n.Lines().Len()
		for i := 0; i < l; i++ {
			line := n.Lines().At(i)
			r.Writer.RawWrite(w, line.Value(source))
		}
	} else {
		_, _ = w.WriteString(`\]</span></p>` + "\n")
	}
	return ast.WalkContinue, nil
}

func (r *mathHTMLRenderer) renderInlineMath(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<span class="math inline">\(`)
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			segment := c.(*ast.Text).Segment
			value := segment.Value(source)
			if bytes.HasSuffix(value, []byte("\n")) {
				r.Writer.RawWrite(w, bytes.TrimSuffix(value[:len(value)-1], []byte("\r")))
				if c != n.LastChild() {
					_, _ = w.Write([]byte(" "))
				}
			} else {
				r.Writer.RawWrite(w, value)
			}
		}
		return ast.WalkSkipChildren, nil
	}
	_, _ = w.WriteString(`\)</span>`)
	return ast.WalkContinue, nil
}

type mathjaxExtender struct{}

func (e *mathjaxExtender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithBlockParsers(
			util.Prioritized(&mathBlockParser{}, 701),
		),
		parser.WithInlineParsers(
			util.Prioritized(&inlineMathParser{}, 501),
		),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(&mathHTMLRenderer{Config: gmhtml.NewConfig()}, 501),
		),
	)
}
