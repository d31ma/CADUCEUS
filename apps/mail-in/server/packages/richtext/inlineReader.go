package richtext

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

type inlineReader struct {
	builder  *blockBuilder
	text     string
	base     Span
	open     map[string]bool
	results  map[string]searchResult
	handlers map[byte]func(int) int
}

func newInlineReader(builder *blockBuilder, base Span) *inlineReader {
	reader := &inlineReader{builder: builder, base: base, open: map[string]bool{}, results: map[string]searchResult{}}
	reader.handlers = map[byte]func(int) int{
		'\\': reader.escape,
		'`':  reader.codeSpan,
		'*':  reader.emphasis,
		'_':  reader.emphasis,
		'~':  reader.emphasis,
		'[':  reader.link,
		'!':  reader.image,
		'<':  reader.autolink,
	}
	return reader
}

func (inlineReader) specials() string {
	return "\\`*_~[!<"
}

func (inlineReader) punctuation() string {
	return "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
}

func (inlineReader) boldMarkers() []string {
	return []string{"**", "__"}
}

func (inlineReader) italicMarkers() []string {
	return []string{"*", "_"}
}

func (inlineReader) strikeMarkers() []string {
	return []string{"~~"}
}

func (reader *inlineReader) read(text string) {
	reader.text = text
	position := 0
	for position < len(text) {
		position = reader.step(position)
	}
}

func (reader *inlineReader) step(position int) int {
	return reader.handlerFor(reader.text[position])(position)
}

func (reader *inlineReader) handlerFor(character byte) func(int) int {
	handler, known := reader.handlers[character]
	if !known {
		return reader.literal
	}
	return handler
}

func (reader *inlineReader) literal(position int) int {
	end := reader.nextSpecial(position + 1)
	reader.add(reader.text[position:end])
	return end
}

func (reader *inlineReader) nextSpecial(from int) int {
	index := strings.IndexAny(reader.text[from:], reader.specials())
	if index < 0 {
		return len(reader.text)
	}
	return from + index
}

func (reader *inlineReader) add(text string) {
	reader.builder.addText(text, reader.style())
}

func (reader *inlineReader) style() Span {
	style := reader.base
	style.Bold = style.Bold || reader.anyOpen(reader.boldMarkers())
	style.Italic = style.Italic || reader.anyOpen(reader.italicMarkers())
	style.Strike = style.Strike || reader.anyOpen(reader.strikeMarkers())
	return style
}

func (reader *inlineReader) anyOpen(markers []string) bool {
	return slices.ContainsFunc(markers, reader.isOpen)
}

func (reader *inlineReader) isOpen(marker string) bool {
	return reader.open[marker]
}

func (reader *inlineReader) find(target string, from int) int {
	from = min(from, len(reader.text))
	result, known := reader.results[target]
	if known && result.covers(from) {
		return result.index
	}
	result = searchResult{from: from, index: reader.absolute(strings.Index(reader.text[from:], target), from)}
	reader.results[target] = result
	return result.index
}

func (reader *inlineReader) absolute(index int, from int) int {
	if index < 0 {
		return index
	}
	return from + index
}

func (reader *inlineReader) escape(position int) int {
	next := reader.text[position+1:]
	if next == "" || !strings.ContainsRune(reader.punctuation(), rune(next[0])) {
		reader.add(reader.text[position : position+1])
		return position + 1
	}
	reader.add(next[:1])
	return position + len("\\") + 1
}

func (reader *inlineReader) codeSpan(position int) int {
	rest := reader.text[position:]
	run := rest[:len(rest)-len(strings.TrimLeft(rest, "`"))]
	start := position + len(run)
	end := reader.find(run, start)
	if end < 0 {
		reader.add(run)
		return start
	}
	reader.addCode(reader.text[start:end])
	return end + len(run)
}

func (reader *inlineReader) addCode(content string) {
	style := reader.style()
	style.Code = true
	reader.builder.addText(reader.codeText(strings.ReplaceAll(content, "\n", " ")), style)
}

func (reader *inlineReader) codeText(content string) string {
	if len(content) < len("  ") || !strings.HasPrefix(content, " ") || !strings.HasSuffix(content, " ") {
		return content
	}
	return content[1 : len(content)-1]
}

func (reader *inlineReader) emphasis(position int) int {
	marker := reader.markerAt(position)
	reader.applyMarker(marker, reader.closes(marker, position) || reader.opens(marker, position))
	return position + len(marker)
}

func (reader *inlineReader) markerAt(position int) string {
	single := reader.text[position : position+1]
	if !strings.HasPrefix(reader.text[position+1:], single) {
		return single
	}
	return single + single
}

func (reader *inlineReader) applyMarker(marker string, toggles bool) {
	if !toggles {
		reader.add(marker)
		return
	}
	reader.open[marker] = !reader.open[marker]
}

func (reader *inlineReader) closes(marker string, position int) bool {
	return reader.open[marker] && reader.textBefore(position)
}

func (reader *inlineReader) opens(marker string, position int) bool {
	after := position + len(marker)
	return !reader.open[marker] && reader.isMarker(marker) && reader.textAfter(after) &&
		reader.find(marker, after) >= 0 && reader.mayOpen(marker, position)
}

func (reader *inlineReader) isMarker(marker string) bool {
	markers := slices.Concat(reader.boldMarkers(), reader.italicMarkers(), reader.strikeMarkers())
	return slices.Contains(markers, marker)
}

func (reader *inlineReader) textBefore(position int) bool {
	character, size := utf8.DecodeLastRuneInString(reader.text[:position])
	return size > 0 && !unicode.IsSpace(character)
}

func (reader *inlineReader) textAfter(position int) bool {
	character, size := utf8.DecodeRuneInString(reader.text[position:])
	return size > 0 && !unicode.IsSpace(character)
}

func (reader *inlineReader) mayOpen(marker string, position int) bool {
	character, _ := utf8.DecodeLastRuneInString(reader.text[:position])
	return !strings.HasPrefix(marker, "_") || !(unicode.IsLetter(character) || unicode.IsDigit(character))
}

func (reader *inlineReader) link(position int) int {
	target := reader.linkAt(position)
	if !target.found {
		reader.add(reader.text[position : position+1])
		return position + 1
	}
	style := reader.style()
	style.Href = schemes{}.link(target.address)
	newInlineReader(reader.builder, style).read(target.label)
	return target.end
}

func (reader *inlineReader) linkAt(position int) inlineLink {
	length := len(reader.text)
	closing := reader.find("]", position+1)
	open := min(max(closing, position)+1, length)
	end := reader.find(")", open+1)
	labelEnd := max(closing, min(position+1, length))
	addressStart := min(open+1, length)
	return inlineLink{
		label:   reader.text[min(position+1, labelEnd):labelEnd],
		address: reader.addressOf(reader.text[addressStart:max(end, addressStart)]),
		end:     end + 1,
		found:   closing >= 0 && strings.HasPrefix(reader.text[open:], "(") && end >= 0,
	}
}

func (reader *inlineReader) addressOf(text string) string {
	address, _, _ := strings.Cut(strings.TrimSpace(text), " ")
	return strings.TrimSuffix(strings.TrimPrefix(address, "<"), ">")
}

func (reader *inlineReader) image(position int) int {
	target := reader.linkAt(position + 1)
	if !strings.HasPrefix(reader.text[position+1:], "[") || !target.found {
		reader.add(reader.text[position : position+1])
		return position + 1
	}
	reader.builder.addImage(ImageRef{Alt: target.label, RemoteUrl: schemes{}.image(target.address)})
	return reader.skipSpaces(target.end)
}

func (reader *inlineReader) skipSpaces(position int) int {
	rest := reader.text[position:]
	return position + len(rest) - len(strings.TrimLeft(rest, " \t\n"))
}

func (reader *inlineReader) autolink(position int) int {
	end := reader.find(">", position+1)
	address := reader.text[position+1 : max(end, position+1)]
	if end < 0 || !reader.plainBetween(position+1, end) || (schemes{}).link(address) == "" {
		reader.add(reader.text[position : position+1])
		return position + 1
	}
	style := reader.style()
	style.Href = address
	reader.builder.addText(address, style)
	return end + 1
}

func (reader *inlineReader) plainBetween(from int, end int) bool {
	return !slices.ContainsFunc([]string{" ", "\t", "\n", "<"}, func(target string) bool {
		return reader.foundBefore(target, from, end)
	})
}

func (reader *inlineReader) foundBefore(target string, from int, end int) bool {
	index := reader.find(target, from)
	return index >= 0 && index < end
}
