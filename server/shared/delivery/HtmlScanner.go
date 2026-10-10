import (
	"html"
	"strings"
)

// HtmlScanner finds the anchors of an HTML body with a small tag scanner:
// the standard library has no HTML parser. It tracks the open elements so an
// anchor knows whether it sits inside quoted content (a blockquote, a
// type="cite" element, or a mail client's quote class), and collects each
// anchor's text as htmlparser2's textContent did.
type HtmlScanner struct {
	source   string
	position int
	elements []bool
	names    []string
	anchors  []HtmlAnchor
	open     []int
}

// Anchors answers the <a href> elements of source in document order.
func (HtmlScanner) Anchors(source string) []HtmlAnchor {
	scanner := &HtmlScanner{source: source}
	for scanner.position < len(source) {
		next := strings.IndexByte(source[scanner.position:], '<')
		if next < 0 {
			scanner.text(source[scanner.position:])
			break
		}
		scanner.text(source[scanner.position : scanner.position+next])
		scanner.position += next
		scanner.markup()
	}
	return scanner.anchors
}

func (scanner *HtmlScanner) text(run string) {
	if run == "" || len(scanner.open) == 0 {
		return
	}
	decoded := html.UnescapeString(run)
	for _, index := range scanner.open {
		scanner.anchors[index].Text += decoded
	}
}

// markup consumes the markup at the scanner's position, which is a '<'.
func (scanner *HtmlScanner) markup() {
	rest := scanner.source[scanner.position:]
	switch {
	case strings.HasPrefix(rest, "<!--"):
		scanner.skipPast("-->", 4)
	case len(rest) > 1 && (rest[1] == '!' || rest[1] == '?'):
		scanner.skipPast(">", 1)
	case len(rest) > 1 && rest[1] == '/':
		scanner.closing()
	case len(rest) > 1 && scanner.letter(rest[1]):
		scanner.opening()
	default:
		scanner.text("<")
		scanner.position++
	}
}

func (scanner *HtmlScanner) skipPast(marker string, from int) {
	end := strings.Index(scanner.source[scanner.position+from:], marker)
	if end < 0 {
		scanner.position = len(scanner.source)
		return
	}
	scanner.position += from + end + len(marker)
}

func (scanner *HtmlScanner) closing() {
	start := scanner.position + 2
	name := strings.ToLower(scanner.name(start))
	scanner.skipPast(">", 2)
	for index := len(scanner.names) - 1; index >= 0; index-- {
		if scanner.names[index] == name {
			scanner.names, scanner.elements = scanner.names[:index], scanner.elements[:index]
			break
		}
	}
	if name == "a" && len(scanner.open) > 0 {
		scanner.open = scanner.open[:len(scanner.open)-1]
	}
}

func (scanner *HtmlScanner) opening() {
	start := scanner.position + 1
	name := strings.ToLower(scanner.name(start))
	cursor := start + len(name)
	attributes := map[string]string{}
	hrefStart, hrefEnd, selfClosing := -1, -1, false
	for cursor < len(scanner.source) {
		cursor = scanner.skipSpace(cursor)
		if cursor >= len(scanner.source) || scanner.source[cursor] == '>' {
			break
		}
		if scanner.source[cursor] == '/' {
			selfClosing = true
			cursor++
			continue
		}
		selfClosing = false
		attributeStart := cursor
		attribute, value, end := scanner.attribute(cursor)
		if _, seen := attributes[attribute]; !seen {
			attributes[attribute] = value
			if attribute == "href" {
				hrefStart, hrefEnd = attributeStart, end
			}
		}
		cursor = end
	}
	scanner.position = cursor + 1
	quoted := scanner.quoting()
	if !selfClosing && !scanner.void(name) {
		scanner.names = append(scanner.names, name)
		scanner.elements = append(scanner.elements, scanner.quotes(name, attributes))
	}
	if name == "a" {
		if href, hasHref := attributes["href"]; hasHref {
			scanner.anchors = append(scanner.anchors, HtmlAnchor{Href: href, Quoted: quoted, HrefStart: hrefStart, HrefEnd: hrefEnd})
			if !selfClosing {
				scanner.open = append(scanner.open, len(scanner.anchors)-1)
			}
		}
	}
	if scanner.raw(name) {
		scanner.skipRaw(name)
	}
}

// attribute reads one attribute at cursor: its lower-case name, decoded
// value, and the position after it.
func (scanner *HtmlScanner) attribute(cursor int) (string, string, int) {
	source := scanner.source
	start := cursor
	for cursor < len(source) && !strings.ContainsRune(" \t\r\n\f=>/", rune(source[cursor])) {
		cursor++
	}
	if cursor == start {
		cursor++
	}
	name := strings.ToLower(source[start:cursor])
	after := scanner.skipSpace(cursor)
	if after >= len(source) || source[after] != '=' {
		return name, "", cursor
	}
	cursor = scanner.skipSpace(after + 1)
	if cursor < len(source) && (source[cursor] == '"' || source[cursor] == '\'') {
		end := strings.IndexByte(source[cursor+1:], source[cursor])
		if end < 0 {
			return name, html.UnescapeString(source[cursor+1:]), len(source)
		}
		return name, html.UnescapeString(source[cursor+1 : cursor+1+end]), cursor + end + 2
	}
	valueStart := cursor
	for cursor < len(source) && !strings.ContainsRune(" \t\r\n\f>", rune(source[cursor])) {
		cursor++
	}
	return name, html.UnescapeString(source[valueStart:cursor]), cursor
}

func (scanner *HtmlScanner) name(start int) string {
	end := start
	for end < len(scanner.source) && !strings.ContainsRune(" \t\r\n\f/>", rune(scanner.source[end])) {
		end++
	}
	return scanner.source[start:end]
}

func (scanner *HtmlScanner) skipSpace(cursor int) int {
	for cursor < len(scanner.source) && strings.ContainsRune(" \t\r\n\f", rune(scanner.source[cursor])) {
		cursor++
	}
	return cursor
}

func (scanner *HtmlScanner) skipRaw(name string) {
	end := strings.Index(strings.ToLower(scanner.source[scanner.position:]), "</"+name)
	if end < 0 {
		scanner.position = len(scanner.source)
		return
	}
	scanner.position += end
}

// quoting reports whether an element opened now sits inside quoted content.
func (scanner *HtmlScanner) quoting() bool {
	for _, quotes := range scanner.elements {
		if quotes {
			return true
		}
	}
	return false
}

func (HtmlScanner) quotes(name string, attributes map[string]string) bool {
	if name == "blockquote" || strings.ToLower(attributes["type"]) == "cite" {
		return true
	}
	for _, class := range strings.Fields(attributes["class"]) {
		switch strings.ToLower(class) {
		case "gmail_quote", "gmail_extra", "yahoo_quoted", "moz-cite-prefix":
			return true
		}
	}
	return false
}

func (HtmlScanner) letter(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
}

func (HtmlScanner) void(name string) bool {
	switch name {
	case "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr":
		return true
	}
	return false
}

func (HtmlScanner) raw(name string) bool {
	return name == "script" || name == "style" || name == "textarea" || name == "title"
}
