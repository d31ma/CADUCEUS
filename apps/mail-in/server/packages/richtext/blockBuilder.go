package richtext

import "strings"

type blockBuilder struct {
	blocks     []Block
	shape      Block
	spans      []Span
	style      Span
	text       strings.Builder
	cells      []string
	rowOpen    bool
	cellOpen   bool
	spaces     int
	breaks     int
	characters int
	truncated  bool
}

func newBlockBuilder() *blockBuilder {
	return &blockBuilder{blocks: []Block{}, shape: Block{Kind: Kinds{}.Paragraph()}, spans: []Span{}, cells: []string{}}
}

func (builder *blockBuilder) setShape(shape Block) {
	if builder.rowOpen {
		builder.space()
		return
	}
	builder.commit()
	builder.shape = shape
}

func (builder *blockBuilder) space() {
	builder.spaces = 1
}

func (builder *blockBuilder) newline() {
	if builder.rowOpen {
		builder.space()
		return
	}
	builder.breaks++
}

func (builder *blockBuilder) addText(text string, style Span) {
	if text == "" {
		return
	}
	builder.write(builder.fit(builder.pendingSeparator()), builder.style)
	builder.write(builder.fit(text), style)
}

func (builder *blockBuilder) addFlowing(text string, style Span) {
	builder.spaceWhen(strings.TrimLeftFunc(text, builder.isSpace) != text)
	builder.addText(strings.Join(strings.FieldsFunc(text, builder.isSpace), " "), style)
	builder.spaceWhen(strings.TrimRightFunc(text, builder.isSpace) != text)
}

func (builder *blockBuilder) spaceWhen(found bool) {
	if found {
		builder.space()
	}
}

func (builder *blockBuilder) isSpace(character rune) bool {
	return strings.ContainsRune(" \t\n\f\r", character)
}

func (builder *blockBuilder) addLines(lines []string, style Span) {
	builder.addText(lines[0], style)
	for _, line := range lines[1:] {
		builder.newline()
		builder.addText(line, style)
	}
}

func (builder *blockBuilder) addImage(image ImageRef) {
	if image == (ImageRef{}) {
		return
	}
	image.Alt = builder.fit(image.Alt)
	builder.addBlock(Block{Kind: Kinds{}.Image(), Spans: []Span{}, Image: &image, Cells: []string{}})
}

func (builder *blockBuilder) addDivider() {
	builder.addBlock(Block{Kind: Kinds{}.Divider(), Spans: []Span{}, Cells: []string{}})
}

func (builder *blockBuilder) addBlock(block Block) {
	if builder.rowOpen {
		builder.addText(block.plainText(), builder.style)
		return
	}
	builder.commit()
	builder.appendBlock(block)
}

func (builder *blockBuilder) startRow() {
	builder.commit()
	builder.rowOpen = true
}

func (builder *blockBuilder) startCell() {
	builder.finishCell()
	builder.cellOpen = true
	builder.spaces = 0
	builder.breaks = 0
}

func (builder *blockBuilder) endRow() {
	builder.commit()
}

func (builder *blockBuilder) result() Result {
	builder.commit()
	return Result{Blocks: builder.blocks, Text: builder.plainText(), Truncated: builder.truncated}
}

func (blockBuilder) lineBreak() string {
	return "\n"
}

func (builder *blockBuilder) pendingSeparator() string {
	separator := builder.separator()
	builder.spaces = 0
	builder.breaks = 0
	return builder.onlyAfterText(separator)
}

func (builder *blockBuilder) separator() string {
	if builder.breaks > 0 {
		return strings.Repeat(builder.lineBreak(), builder.breaks)
	}
	return strings.Repeat(" ", builder.spaces)
}

func (builder *blockBuilder) onlyAfterText(separator string) string {
	if builder.text.Len()+len(builder.spans) == 0 {
		return ""
	}
	return separator
}

func (builder *blockBuilder) write(text string, style Span) {
	if builder.rowOpen {
		builder.writeCell(text)
		return
	}
	builder.writeSpan(text, style)
}

func (builder *blockBuilder) writeCell(text string) {
	builder.cellOpen = true
	builder.text.WriteString(text)
}

func (builder *blockBuilder) writeSpan(text string, style Span) {
	builder.changeStyle(style)
	builder.text.WriteString(text)
}

func (builder *blockBuilder) changeStyle(style Span) {
	if style == builder.style {
		return
	}
	builder.finishSpan()
	builder.style = style
}

func (builder *blockBuilder) finishSpan() {
	if builder.text.Len() == 0 {
		return
	}
	span := builder.style
	span.Text = builder.text.String()
	builder.spans = append(builder.spans, span)
	builder.text.Reset()
}

func (builder *blockBuilder) fit(text string) string {
	runes := []rune(text)
	kept := min(len(runes), Limits{}.MaxCharacters()-builder.characters)
	builder.characters += kept
	builder.noteCut(kept < len(runes))
	return string(runes[:kept])
}

func (builder *blockBuilder) noteCut(cut bool) {
	if cut {
		builder.truncated = true
	}
}

func (builder *blockBuilder) commit() {
	block := builder.finish()
	builder.spaces = 0
	builder.breaks = 0
	builder.keep(block)
}

func (builder *blockBuilder) finish() Block {
	if builder.rowOpen {
		return builder.finishRow()
	}
	return builder.finishSpans()
}

func (builder *blockBuilder) finishSpans() Block {
	builder.finishSpan()
	block := Block{Kind: builder.shape.Kind, Level: builder.shape.Level, Ordered: builder.shape.Ordered, Spans: builder.spans, Cells: []string{}}
	builder.spans = []Span{}
	return block
}

func (builder *blockBuilder) finishRow() Block {
	builder.finishCell()
	block := Block{Kind: Kinds{}.Row(), Spans: []Span{}, Cells: builder.cells}
	builder.cells = []string{}
	builder.rowOpen = false
	builder.cellOpen = false
	return block
}

func (builder *blockBuilder) finishCell() {
	if !builder.cellOpen {
		return
	}
	builder.cells = append(builder.cells, builder.text.String())
	builder.text.Reset()
}

func (builder *blockBuilder) keep(block Block) {
	if strings.TrimSpace(block.plainText()) == "" {
		return
	}
	builder.appendBlock(block)
}

func (builder *blockBuilder) appendBlock(block Block) {
	if len(builder.blocks) >= (Limits{}).MaxBlocks() {
		builder.truncated = true
		return
	}
	builder.blocks = append(builder.blocks, block)
}

func (builder *blockBuilder) plainText() string {
	texts := make([]string, 0, len(builder.blocks))
	for _, block := range builder.blocks {
		texts = append(texts, block.plainText())
	}
	return strings.Join(texts, builder.lineBreak())
}
