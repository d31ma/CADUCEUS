package richtext

import (
	"strings"

	"golang.org/x/net/html"
)

type Converter struct{}

func (converter Converter) FromHtml(source string, inlineImages map[string]string) Result {
	builder := newBlockBuilder()
	valid := converter.valid(source)
	document, err := html.Parse(strings.NewReader(valid))
	if err != nil {
		newFlatHtmlReader(builder).read(valid)
		return builder.result()
	}
	newHtmlReader(builder, inlineImages).read(document)
	return builder.result()
}

func (converter Converter) FromMarkdown(source string) Result {
	builder := newBlockBuilder()
	newMarkdownReader(builder).read(converter.lines(source))
	return builder.result()
}

func (converter Converter) FromText(text string) Result {
	builder := newBlockBuilder()
	textReader{builder: builder}.read(converter.lines(text))
	return builder.result()
}

func (Converter) valid(text string) string {
	return strings.ToValidUTF8(text, "�")
}

func (converter Converter) lines(text string) string {
	return strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(converter.valid(text))
}
