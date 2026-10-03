package svg

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const open = `<svg xmlns="http://www.w3.org/2000/svg">`

func TestSanitize(t *testing.T) {
	for name, tt := range map[string]struct{ in, want string }{
		"kept as is": {
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><circle cx="16" cy="16" r="16" fill="#2f6feb"></circle></svg>`,
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><circle cx="16" cy="16" r="16" fill="#2f6feb"></circle></svg>`,
		},
		"prolog, comments and processing instructions": {
			`<?xml version="1.0"?>` + "\n<!-- logo -->\n" + open + `<!-- x --><?php echo 1 ?><g/></svg>` + "\n<!-- after -->",
			open + `<g></g></svg>`,
		},
		"text and elements outside the root": {open + `</svg>tail<svg xmlns="http://www.w3.org/2000/svg"><script/></svg>`, open + `</svg>`},
		"prefixed SVG elements": {
			`<s:svg xmlns:s="http://www.w3.org/2000/svg"><s:rect width="1"/></s:svg>`,
			open + `<rect width="1"></rect></svg>`,
		},
		"escaped text and attributes": {
			open + `<text x="&quot;&lt;&amp;">a &lt; b &amp; c &#65;&#x42; <![CDATA[<i>]]></text></svg>`,
			open + `<text x="&quot;&lt;&amp;">a &lt; b &amp; c AB &lt;i&gt;</text></svg>`,
		},
		"text and title elements": {
			open + `<title>Acme</title><desc>Logo</desc><text><tspan>A</tspan></text></svg>`,
			open + `<title>Acme</title><desc>Logo</desc><text><tspan>A</tspan></text></svg>`,
		},
		"gradients, masks and filters": {
			open + `<defs><linearGradient id="g"><stop offset="0"/></linearGradient><filter id="f"><feGaussianBlur stdDeviation="1"/><feDropShadow/></filter></defs><rect fill="url(#g)" filter="url( '#f' )"/></svg>`,
			open + `<defs><linearGradient id="g"><stop offset="0"></stop></linearGradient><filter id="f"><feGaussianBlur stdDeviation="1"></feGaussianBlur><feDropShadow></feDropShadow></filter></defs><rect fill="url(#g)" filter="url( '#f' )"></rect></svg>`,
		},
	} {
		got, err := Sanitize(tt.in)
		require.NoError(t, err, name)
		assert.Equal(t, tt.want, got, name)
	}
}

func TestSanitizeAttacks(t *testing.T) {
	for name, tt := range map[string]struct{ in, want string }{
		"script":          {open + `<script>alert(1)</script><g/></svg>`, open + `<g></g></svg>`},
		"script in CDATA": {open + `<script><![CDATA[alert(1)]]></script></svg>`, open + `</svg>`},
		"foreign object": {
			open + `<foreignObject><div xmlns="http://www.w3.org/1999/xhtml" onclick="x()">hi</div></foreignObject></svg>`,
			open + `</svg>`,
		},
		"elements of another namespace": {
			open + `<h:script xmlns:h="http://www.w3.org/1999/xhtml">x()</h:script><g xmlns="">t</g></svg>`,
			open + `</svg>`,
		},
		"links":           {open + `<a href="https://evil.example"><text>click</text></a></svg>`, open + `</svg>`},
		"images":          {open + `<image href="https://tracker.example/p.png"/></svg>`, open + `</svg>`},
		"animations":      {open + `<rect><set attributeName="href" to="javascript:x()"/><animate attributeName="fill"/></rect></svg>`, open + `<rect></rect></svg>`},
		"event handlers":  {`<svg xmlns="http://www.w3.org/2000/svg" onload="x()"><rect ONCLICK="x()" onMouseOver="x()" width="1"/></svg>`, open + `<rect width="1"></rect></svg>`},
		"external href":   {open + `<use href="https://evil.example/s.svg#a"/><use href=" #a"/><use href="javascript:x()"/></svg>`, open + `<use></use><use></use><use></use></svg>`},
		"fragment href":   {open + `<use href="#a"/></svg>`, open + `<use href="#a"></use></svg>`},
		"xlink:href":      {`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><use xlink:href="#a"/><use xlink:href="data:x"/></svg>`, open + `<use href="#a"></use><use></use></svg>`},
		"duplicate href":  {`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><use href="#a" xlink:href="#b"/></svg>`, open + `<use href="#a"></use></svg>`},
		"namespaced":      {`<svg xmlns="http://www.w3.org/2000/svg" xmlns:x="urn:x" x:a="1" xml:space="preserve"><g x:b="2"/></svg>`, open + `<g></g></svg>`},
		"url in style":    {open + `<rect style="fill: url(https://tracker.example/p)" stroke="URL( 'data:x' )" fill="url(#ok)"/></svg>`, open + `<rect fill="url(#ok)"></rect></svg>`},
		"image-set":       {open + `<rect style="background: -webkit-image-set('a.png' 1x)"/></svg>`, open + `<rect></rect></svg>`},
		"import in style": {open + `<style>@import "https://evil.example/a.css"; rect { fill: red }</style></svg>`, open + `<style></style></svg>`},
		"url in style element": {
			open + `<style>rect { fill: url(#g) } circle { background: url("https://tracker.example/p") }</style></svg>`,
			open + `<style></style></svg>`,
		},
		"escapes in style element": {open + `<style>rect { background: u\72l(x) }</style></svg>`, open + `<style></style></svg>`},
		"backslash":                {open + `<rect style="fill: u\72l(x)"/></svg>`, open + `<rect></rect></svg>`},
		"safe style element":       {open + `<style>rect { fill: url(#g) &gt; }<g/></style></svg>`, open + `<style>rect { fill: url(#g) &gt; }</style></svg>`},
	} {
		got, err := Sanitize(tt.in)
		require.NoError(t, err, name)
		assert.Equal(t, tt.want, got, name)
	}
}

func TestSanitizeRejects(t *testing.T) {
	for name, in := range map[string]string{
		"text":                      "logo",
		"empty":                     "",
		"unclosed":                  open + `<g>`,
		"mismatched":                open + `<g></rect></svg>`,
		"HTML root":                 `<html><svg xmlns="http://www.w3.org/2000/svg"/></html>`,
		"root without namespace":    `<svg></svg>`,
		"root of another namespace": `<svg xmlns="http://www.w3.org/1999/xhtml"></svg>`,
		"DTD":                       `<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">` + open + `</svg>`,
		"entity declaration":        `<!DOCTYPE svg [<!ENTITY x "y">]>` + open + `&x;</svg>`,
		"billion laughs":            `<!DOCTYPE svg [<!ENTITY a "aa"><!ENTITY b "&a;&a;">]>` + open + `&b;</svg>`,
		"external entity":           `<!DOCTYPE svg [<!ENTITY x SYSTEM "file:///etc/passwd">]>` + open + `&x;</svg>`,
		"undeclared entity":         open + `&nbsp;</svg>`,
	} {
		_, err := Sanitize(in)
		assert.ErrorIs(t, err, ErrInvalid, name)
	}
}

func TestSanitizeSize(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	wrap := func(n int) string {
		fill := n - len(open) - len(`<path d=""></path></svg>`)
		return open + `<path d="` + strings.Repeat("M", fill) + `"></path></svg>`
	}

	got, err := Sanitize(wrap(MaxSize))
	require.NoError(err)
	assert.Len(got, MaxSize, "an image of exactly the limit is kept")
	_, err = Sanitize(wrap(MaxSize + 1))
	assert.ErrorIs(err, ErrTooLarge)
	_, err = Sanitize(open + "<!--" + strings.Repeat("x", MaxSize) + "--></svg>")
	assert.NoError(err, "the limit applies to the sanitized image")
}

func TestSanitizeIdempotent(t *testing.T) {
	for _, in := range []string{
		`<?xml version="1.0"?><s:svg xmlns:s="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 1 1">` +
			"\n  <s:style>rect { fill: url(#g) }\n</s:style><s:text x='a\"b'>a &lt; b &amp; &#10; c</s:text>" +
			`<s:use xlink:href="#a" onload="x()"/><s:script>x()</s:script></s:svg>`,
		open + "<text>\r\n\ttabs\tand\r\nlines</text><rect fill=\"a\tb\"/></svg>",
	} {
		once, err := Sanitize(in)
		require.NoError(t, err)
		twice, err := Sanitize(once)
		require.NoError(t, err)
		assert.Equal(t, once, twice)
	}
}
