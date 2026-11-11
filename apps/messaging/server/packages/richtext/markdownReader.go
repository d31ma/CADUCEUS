package richtext

import (
	"slices"
	"strings"
)

type markdownReader struct {
	builder     *blockBuilder
	lines       []string
	shape       Block
	depth       int
	open        bool
	fence       string
	fenceDepth  int
	indented    bool
	listIndents []int
}

func newMarkdownReader(builder *blockBuilder) *markdownReader {
	return &markdownReader{builder: builder, lines: []string{}, shape: Block{Kind: Kinds{}.Paragraph()}, listIndents: []int{}}
}

func (reader *markdownReader) read(source string) {
	for _, line := range strings.Split(source, reader.lineBreak()) {
		reader.readLine(reader.quoted(line, len(line)))
	}
	reader.closeOpen()
}

func (reader *markdownReader) readLine(line markdownLine) {
	rules := reader.rules(line)
	rules[slices.IndexFunc(rules, markdownRule.isActive)].handle(line)
}

func (reader *markdownReader) rules(line markdownLine) []markdownRule {
	content := line.content
	return []markdownRule{
		{active: reader.fence != "", handle: reader.fenceLine},
		{active: reader.fenceOf(content) != "", handle: reader.startFence},
		{active: reader.indented && reader.indentedOrBlank(content), handle: reader.indentedLine},
		{active: reader.isBlank(content), handle: reader.blankLine},
		{active: reader.headingLevel(content) > 0, handle: reader.heading},
		{active: reader.isDivider(content), handle: reader.divider},
		{active: listMarker{}.parse(content).found, handle: reader.listItem},
		{active: reader.startsIndentedCode(content), handle: reader.startIndentedCode},
		{active: reader.open && line.depth == reader.depth, handle: reader.continueParagraph},
		{active: true, handle: reader.startParagraph},
	}
}

func (reader *markdownReader) quoted(raw string, limit int) markdownLine {
	line := markdownLine{raw: raw, content: raw}
	for line.depth < limit && reader.startsQuote(line.content) {
		line.content = strings.TrimPrefix(strings.TrimPrefix(strings.TrimLeft(line.content, " "), reader.quoteMarker()), " ")
		line.depth++
	}
	return line
}

func (reader *markdownReader) startsQuote(content string) bool {
	return strings.HasPrefix(strings.TrimLeft(content, " "), reader.quoteMarker())
}

func (reader *markdownReader) isBlank(content string) bool {
	return strings.TrimSpace(content) == ""
}

func (reader *markdownReader) isIndented(content string) bool {
	return strings.HasPrefix(content, reader.codeIndent()) || strings.HasPrefix(content, "\t")
}

func (reader *markdownReader) indentedOrBlank(content string) bool {
	return reader.isIndented(content) || reader.isBlank(content)
}

func (reader *markdownReader) startsIndentedCode(content string) bool {
	return !reader.open && len(reader.listIndents) == 0 && reader.isIndented(content)
}

func (markdownReader) lineBreak() string {
	return "\n"
}

func (markdownReader) quoteMarker() string {
	return ">"
}

func (markdownReader) headingMarker() string {
	return "#"
}

func (markdownReader) codeIndent() string {
	return "    "
}

func (markdownReader) backtick() string {
	return "`"
}

func (markdownReader) fenceLength() int {
	return 3
}

func (markdownReader) maxHeadingLevel() int {
	return 6
}

func (reader *markdownReader) fenceOf(content string) string {
	trimmed := strings.TrimLeft(content, " ")
	run := reader.runOf(trimmed, reader.backtick()) + reader.runOf(trimmed, "~")
	if len(run) < reader.fenceLength() || strings.Contains(trimmed[len(run):], reader.backtick()) {
		return ""
	}
	return run
}

func (reader *markdownReader) runOf(text string, character string) string {
	return text[:len(text)-len(strings.TrimLeft(text, character))]
}

func (reader *markdownReader) startFence(line markdownLine) {
	reader.closeOpen()
	reader.resetLists()
	reader.fence = reader.fenceOf(line.content)
	reader.fenceDepth = line.depth
	reader.builder.setShape(Block{Kind: Kinds{}.Code()})
}

func (reader *markdownReader) fenceLine(line markdownLine) {
	content := reader.quoted(line.raw, reader.fenceDepth).content
	if reader.closesFence(content) {
		reader.fence = ""
		return
	}
	reader.addCode(content)
}

func (reader *markdownReader) closesFence(content string) bool {
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, reader.fence) && strings.Trim(trimmed, reader.fence[:1]) == ""
}

func (reader *markdownReader) startIndentedCode(line markdownLine) {
	reader.closeOpen()
	reader.builder.setShape(Block{Kind: Kinds{}.Code()})
	reader.indented = true
	reader.indentedLine(line)
}

func (reader *markdownReader) indentedLine(line markdownLine) {
	content := strings.TrimPrefix(line.content, "\t")
	reader.addCode(strings.TrimPrefix(content, reader.codeIndent()))
}

func (reader *markdownReader) addCode(content string) {
	reader.builder.newline()
	reader.builder.addText(content, Span{})
}

func (reader *markdownReader) blankLine(markdownLine) {
	reader.closeOpen()
}

func (reader *markdownReader) headingLevel(content string) int {
	trimmed := strings.TrimLeft(content, " ")
	rest := strings.TrimLeft(trimmed, reader.headingMarker())
	level := len(trimmed) - len(rest)
	if level > reader.maxHeadingLevel() || !(listMarker{}).separated(rest) {
		return 0
	}
	return level
}

func (reader *markdownReader) heading(line markdownLine) {
	reader.closeOpen()
	reader.resetLists()
	reader.builder.setShape(Block{Kind: Kinds{}.Heading(), Level: reader.headingLevel(line.content)})
	reader.inline(reader.headingText(strings.TrimSpace(strings.TrimLeft(strings.TrimLeft(line.content, " "), reader.headingMarker()))))
}

func (reader *markdownReader) headingText(text string) string {
	closed := strings.TrimRight(text, reader.headingMarker())
	if closed != "" && !strings.HasSuffix(closed, " ") {
		return text
	}
	return strings.TrimSpace(closed)
}

func (reader *markdownReader) isDivider(content string) bool {
	compact := strings.NewReplacer(" ", "", "\t", "").Replace(content)
	return len(compact) >= reader.fenceLength() && strings.Trim(compact, compact[:1]) == "" && strings.Contains("-*_", compact[:1])
}

func (reader *markdownReader) divider(markdownLine) {
	reader.closeOpen()
	reader.resetLists()
	reader.builder.addDivider()
}

func (reader *markdownReader) listItem(line markdownLine) {
	marker := listMarker{}.parse(line.content)
	reader.closeOpen()
	level := reader.nest(marker.indent)
	reader.begin(Block{Kind: Kinds{}.ListItem(), Level: level, Ordered: marker.ordered}, line.depth, marker.content)
}

func (reader *markdownReader) nest(indent int) int {
	for len(reader.listIndents) > 0 && reader.listIndents[len(reader.listIndents)-1] >= indent {
		reader.listIndents = reader.listIndents[:len(reader.listIndents)-1]
	}
	reader.listIndents = append(reader.listIndents, indent)
	return min(len(reader.listIndents), Limits{}.MaxDepth())
}

func (reader *markdownReader) startParagraph(line markdownLine) {
	reader.closeOpen()
	reader.resetLists()
	reader.begin(reader.paragraphShape(line.depth), line.depth, strings.TrimLeft(line.content, listMarker{}.blanks()))
}

func (reader *markdownReader) paragraphShape(depth int) Block {
	if depth == 0 {
		return Block{Kind: Kinds{}.Paragraph()}
	}
	return Block{Kind: Kinds{}.Quote(), Level: min(depth, Limits{}.MaxDepth())}
}

func (reader *markdownReader) continueParagraph(line markdownLine) {
	reader.lines = append(reader.lines, strings.TrimLeft(line.content, listMarker{}.blanks()))
}

func (reader *markdownReader) begin(shape Block, depth int, firstLine string) {
	reader.shape = shape
	reader.depth = depth
	reader.open = true
	reader.lines = append(reader.lines, firstLine)
}

func (reader *markdownReader) closeOpen() {
	reader.builder.setShape(reader.shape)
	reader.inline(strings.Join(reader.lines, reader.lineBreak()))
	reader.lines = []string{}
	reader.open = false
	reader.indented = false
}

func (reader *markdownReader) resetLists() {
	reader.listIndents = reader.listIndents[:0]
}

func (reader *markdownReader) inline(text string) {
	newInlineReader(reader.builder, Span{}).read(text)
}
