package richtext

import "strings"

type textReader struct {
	builder *blockBuilder
}

func (reader textReader) read(text string) {
	for _, line := range strings.Split(text, "\n") {
		reader.readLine(strings.TrimRight(line, " \t"))
	}
}

func (reader textReader) readLine(line string) {
	if strings.TrimSpace(line) == "" {
		reader.builder.setShape(Block{Kind: Kinds{}.Paragraph()})
		return
	}
	reader.builder.newline()
	reader.builder.addText(line, Span{})
}
