// Package svg sanitizes SVG images, such as site logos, so that they can be
// served without scripts, references to other resources or other active
// content.
package svg

import (
	"encoding/xml"
	"errors"
	"io"
	"slices"
	"strings"
)

// MaxSize is the size limit of a sanitized image in bytes.
const MaxSize = 64 << 10

const (
	namespace      = "http://www.w3.org/2000/svg"
	xlinkNamespace = "http://www.w3.org/1999/xlink"
)

var (
	// ErrInvalid reports input that is not XML, has a DTD or entities, or
	// whose root is not an SVG element.
	ErrInvalid = errors.New("not an SVG image")
	// ErrTooLarge reports a sanitized image larger than MaxSize.
	ErrTooLarge = errors.New("SVG image too large")
)

// elements are the SVG elements that are kept. Any other element is removed
// with its content.
var elements = []string{
	"svg", "g", "defs", "symbol", "use", "title", "desc", "style", "text", "tspan",
	"path", "rect", "circle", "ellipse", "line", "polyline", "polygon",
	"linearGradient", "radialGradient", "stop", "pattern", "clipPath", "mask",
	"filter", "feBlend", "feColorMatrix", "feComposite", "feFlood",
	"feGaussianBlur", "feMerge", "feMergeNode", "feOffset", "feDropShadow",
}

var (
	textEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	attrEscaper = strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	)
)

// Sanitize returns src with only the allowed elements and attributes,
// without comments, processing instructions and anything outside the root
// element, and re-serialized. Sanitizing its result changes nothing.
func Sanitize(src string) (string, error) {
	d := xml.NewDecoder(strings.NewReader(src))
	var b strings.Builder
	var (
		depth   int  // open elements written
		skip    int  // open elements inside a removed one
		started bool // whether the root element was seen
		style   *strings.Builder
	)
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", ErrInvalid
		}

		switch t := tok.(type) {
		case xml.Directive:
			// A DOCTYPE, which may declare entities.
			return "", ErrInvalid
		case xml.StartElement:
			switch {
			case !started:
				if t.Name.Space != namespace || t.Name.Local != "svg" {
					return "", ErrInvalid
				}

				started = true
			case skip > 0 || depth == 0 || style != nil ||
				t.Name.Space != namespace ||
				!slices.Contains(elements, t.Name.Local):
				skip++
				continue
			}

			b.WriteString("<" + t.Name.Local)
			if depth == 0 {
				b.WriteString(` xmlns="` + namespace + `"`)
			}

			for _, a := range attributes(t.Attr) {
				v := attrEscaper.Replace(a.Value)
				b.WriteString(" " + a.Name.Local + `="` + v + `"`)
			}

			b.WriteString(">")
			depth++
			if t.Name.Local == "style" {
				style = &strings.Builder{}
			}
		case xml.EndElement:
			if skip > 0 {
				skip--
				continue
			}

			if depth == 0 {
				continue
			}

			if style != nil {
				if !unsafeCSS(style.String()) {
					b.WriteString(textEscaper.Replace(style.String()))
				}

				style = nil
			}

			b.WriteString("</" + t.Name.Local + ">")
			depth--
		case xml.CharData:
			switch {
			case skip > 0 || depth == 0:
			case style != nil:
				style.Write(t)
			default:
				b.WriteString(textEscaper.Replace(string(t)))
			}
		}
	}

	if !started {
		return "", ErrInvalid
	}

	if b.Len() > MaxSize {
		return "", ErrTooLarge
	}
	return b.String(), nil
}

// attributes returns the attributes to keep: no namespaced attributes and
// namespace declarations, no event handlers, links only within the
// document, and no CSS that loads resources or hides content behind escapes.
// xlink:href becomes href.
func attributes(in []xml.Attr) []xml.Attr {
	var out []xml.Attr
	for _, a := range in {
		if (a.Name.Space == xlinkNamespace || a.Name.Space == "xlink") &&
			a.Name.Local == "href" {
			a.Name.Space = ""
		}

		name := a.Name.Local
		sameName := func(o xml.Attr) bool { return o.Name.Local == name }
		switch {
		case a.Name.Space != "" || name == "xmlns":
		case strings.HasPrefix(strings.ToLower(name), "on"):
		case name == "href" && !strings.HasPrefix(a.Value, "#"):
		case unsafeCSS(a.Value):
		case slices.ContainsFunc(out, sameName):
		default:
			out = append(out, a)
		}
	}
	return out
}

// unsafeCSS reports whether s contains CSS that can load another resource,
// or a backslash, which CSS escapes start with.
func unsafeCSS(s string) bool {
	s = strings.ToLower(s)
	if strings.Contains(s, "@import") || strings.Contains(s, "image-set(") ||
		strings.Contains(s, `\`) {
		return true
	}

	for {
		i := strings.Index(s, "url(")
		if i < 0 {
			return false
		}

		s = s[i+len("url("):]
		if !strings.HasPrefix(strings.TrimLeft(s, " \t\r\n\f'\""), "#") {
			return true
		}
	}
}
