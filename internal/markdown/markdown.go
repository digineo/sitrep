// Package markdown renders Markdown to HTML that is safe to insert into
// pages: GitHub Flavored Markdown without raw HTML, with links only to
// http, https and mailto targets, and with images replaced by their alt
// text, so that no page loads third-party content.
package markdown

import (
	"bytes"
	"net/url"
	"slices"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(
		renderer.WithNodeRenderers(util.Prioritized(safe{}, 0)),
	),
)

// HTML renders src.
func HTML(src string) string {
	var b bytes.Buffer
	if err := md.Convert([]byte(src), &b); err != nil {
		// Rendering into a buffer does not fail.
		panic(err)
	}
	return b.String()
}

// Text returns the plain text of src with whitespace collapsed.
func Text(src string) string {
	source := []byte(src)
	var b bytes.Buffer
	writeText(&b, md.Parser().Parse(text.NewReader(source)), source)
	return strings.Join(strings.Fields(b.String()), " ")
}

// Excerpt returns the plain text of src cut to limit code points, at a word
// boundary where possible. A cut text ends with an ellipsis.
func Excerpt(src string, limit int) string {
	s := []rune(Text(src))
	if len(s) <= limit {
		return string(s)
	}

	end := limit
	for i := limit - 1; i > 0 && s[limit] != ' '; i-- {
		if s[i] == ' ' {
			end = i
			break
		}
	}
	return strings.TrimRight(string(s[:end]), " ") + "…"
}

// safe overrides goldmark's renderers of links, images and raw HTML.
type safe struct{}

func (safe) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(ast.KindLink, renderLink)
	r.Register(ast.KindAutoLink, renderAutoLink)
	r.Register(ast.KindImage, renderImage)
	r.Register(ast.KindRawHTML, skip)
	r.Register(ast.KindHTMLBlock, skip)
}

// allowed reports whether a link may point to dest.
func allowed(dest []byte) bool {
	u, err := url.Parse(string(dest))
	schemes := []string{"http", "https", "mailto"}
	return err == nil && slices.Contains(schemes, strings.ToLower(u.Scheme))
}

// renderLink renders a link with an allowed target, else only its text.
func renderLink(
	w util.BufWriter,
	source []byte,
	node ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	if !allowed(n.Destination) {
		return ast.WalkContinue, nil
	}

	if !entering {
		_, _ = w.WriteString("</a>")
		return ast.WalkContinue, nil
	}

	_, _ = w.WriteString(`<a href="`)
	_, _ = w.Write(util.EscapeHTML(util.URLEscape(n.Destination, true)))
	if n.Title != nil {
		_, _ = w.WriteString(`" title="`)
		_, _ = w.Write(util.EscapeHTML(resolve(n.Title)))
	}

	_, _ = w.WriteString(`">`)
	return ast.WalkContinue, nil
}

// renderAutoLink renders an autolink with an allowed target, else its
// label as text.
func renderAutoLink(
	w util.BufWriter,
	source []byte,
	node ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	n := node.(*ast.AutoLink)
	if !entering {
		return ast.WalkContinue, nil
	}

	dest := n.URL(source)
	if n.AutoLinkType == ast.AutoLinkEmail &&
		!bytes.HasPrefix(bytes.ToLower(dest), []byte("mailto:")) {
		dest = append([]byte("mailto:"), dest...)
	}
	if !allowed(dest) {
		_, _ = w.Write(util.EscapeHTML(n.Label(source)))
		return ast.WalkContinue, nil
	}

	_, _ = w.WriteString(`<a href="`)
	_, _ = w.Write(util.EscapeHTML(util.URLEscape(dest, false)))
	_, _ = w.WriteString(`">`)
	_, _ = w.Write(util.EscapeHTML(n.Label(source)))
	_, _ = w.WriteString("</a>")
	return ast.WalkContinue, nil
}

// renderImage renders an image's alt text as plain text.
func renderImage(
	w util.BufWriter,
	source []byte,
	node ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	if entering {
		var b bytes.Buffer
		writeText(&b, node, source)
		_, _ = w.Write(util.EscapeHTML(b.Bytes()))
	}
	return ast.WalkSkipChildren, nil
}

func skip(util.BufWriter, []byte, ast.Node, bool) (ast.WalkStatus, error) {
	return ast.WalkSkipChildren, nil
}

// writeText writes the text of a node and its descendants, with escapes
// and entity references resolved, and a space between blocks.
func writeText(b *bytes.Buffer, node ast.Node, source []byte) {
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			if n.Type() == ast.TypeBlock {
				b.WriteByte(' ')
			}
			return ast.WalkContinue, nil
		}

		switch n := n.(type) {
		case *ast.Text:
			v := n.Segment.Value(source)
			if n.Parent().Kind() != ast.KindCodeSpan {
				v = resolve(v)
			}
			b.Write(v)
			if n.SoftLineBreak() || n.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(n.Value)
		case *ast.AutoLink:
			b.Write(n.Label(source))
		case *ast.RawHTML, *ast.HTMLBlock:
			return ast.WalkSkipChildren, nil
		case *ast.CodeBlock, *ast.FencedCodeBlock:
			lines := n.Lines()
			for i := range lines.Len() {
				line := lines.At(i)
				b.Write(line.Value(source))
			}
		}
		return ast.WalkContinue, nil
	})
}

func resolve(v []byte) []byte {
	resolved := util.ResolveNumericReferences(util.ResolveEntityNames(v))
	return util.UnescapePunctuations(resolved)
}
