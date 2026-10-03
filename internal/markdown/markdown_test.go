package markdown

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTML(t *testing.T) {
	for src, want := range map[string]string{
		"**bold** and ~~gone~~":             "<p><strong>bold</strong> and <del>gone</del></p>\n",
		"[docs](https://example.com/a?b=c)": `<p><a href="https://example.com/a?b=c">docs</a></p>` + "\n",
		"[mail](mailto:ops@example.com)":    `<p><a href="mailto:ops@example.com">mail</a></p>` + "\n",
		"see www.example.com":               `<p>see <a href="http://www.example.com">www.example.com</a></p>` + "\n",
		"ops@example.com":                   `<p><a href="mailto:ops@example.com">ops@example.com</a></p>` + "\n",
		"- [x] done":                        `<ul>` + "\n" + `<li><input checked="" disabled="" type="checkbox"> done</li>` + "\n</ul>\n",
		"| a |\n|---|\n| 1 |":               "<table>\n<thead>\n<tr>\n<th>a</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n<td>1</td>\n</tr>\n</tbody>\n</table>\n",
	} {
		assert.Equal(t, want, HTML(src), src)
	}
}

func TestHTMLAttacks(t *testing.T) {
	for src, want := range map[string]string{
		// Raw HTML is dropped, inline and as block.
		"a <script>alert(1)</script> b":          "<p>a alert(1) b</p>\n",
		"<div onclick=\"alert(1)\">x</div>":      "",
		"<img src=x onerror=alert(1)>":           "",
		"a <!-- comment --> b":                   "<p>a  b</p>\n",
		"<iframe src=\"https://evil\"></iframe>": "",
		// Links to other schemes render as text.
		"[x](javascript:alert(1))":           "<p>x</p>\n",
		"[x](JavaScript:alert(1))":           "<p>x</p>\n",
		"[x](&#106;avascript:alert(1))":      "<p>x</p>\n",
		"[x](vbscript:msgbox)":               "<p>x</p>\n",
		"[x](data:text/html,<script>)":       "<p>x</p>\n",
		"[x](data:text/html;base64,PHNjcj4)": "<p>x</p>\n",
		"[x](/relative)":                     "<p>x</p>\n",
		"[x](#fragment)":                     "<p>x</p>\n",
		"<javascript:alert(1)>":              "<p>javascript:alert(1)</p>\n",
		"<ftp://example.com>":                "<p>ftp://example.com</p>\n",
		// Attributes cannot be broken out of.
		`[x](https://e.com "a&quot; onmouseover=&quot;alert(1)")`: `<p><a href="https://e.com" title="a&quot; onmouseover=&quot;alert(1)">x</a></p>` + "\n",
		`[x](https://e.com/"onmouseover="alert(1))`:               `<p><a href="https://e.com/%22onmouseover=%22alert(1)">x</a></p>` + "\n",
		// Images are not loaded; their alt text remains as text.
		"![a <b>chart</b>](https://tracker.example/pixel.png)": "<p>a chart</p>\n",
		"![**bold** alt](x.png)":                               "<p>bold alt</p>\n",
		"[![logo](https://e.com/l.png)](https://e.com)":        `<p><a href="https://e.com">logo</a></p>` + "\n",
	} {
		assert.Equal(t, want, HTML(src), src)
	}
}

func TestText(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(
		"Title Some text with a link and code.",
		Text("# Title\n\nSome *text*\nwith [a link](https://e.com) and `code`."),
	)
	assert.Equal(
		"Fish & chips © * a\\*b",
		Text("Fish &amp; chips &copy; \\* `a\\*b`"),
	)
	assert.Equal(
		"one two inline alt",
		Text("- one\n- two\n\n<div>\nblock\n</div>\n\n<b>inline</b>\n\n![alt](x.png)"),
	)
	assert.Equal("", Text("  \n\n"))
}

func TestExcerpt(t *testing.T) {
	assert := assert.New(t)

	assert.Equal("Short text", Excerpt("Short\n\ntext", 200))

	exact := strings.Repeat("a", 200)
	got := Excerpt(exact, 200)
	assert.Equal(exact, got, "a text of exactly the limit is not cut")

	got = Excerpt("one two three", 9)
	assert.Equal("one two…", got, "cut at the last word boundary")

	got = Excerpt("one two three", 8)
	assert.Equal("one two…", got, "a cut at a space keeps the whole word")

	got = Excerpt("abcdefghijk", 8)
	assert.Equal("abcdefgh…", got, "a single long word is cut inside")

	got = Excerpt("äöü äöü äöü", 9)
	assert.Equal("äöü äöü…", got, "limits count code points, not bytes")
}
