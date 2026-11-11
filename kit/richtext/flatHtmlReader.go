package richtext

import (
	"strings"

	"golang.org/x/net/html"
)

type flatHtmlReader struct {
	builder  *blockBuilder
	dropped  int
	starts   map[string]func()
	ends     map[string]func()
	handlers map[html.TokenType]func(html.Token)
}

func newFlatHtmlReader(builder *blockBuilder) *flatHtmlReader {
	reader := &flatHtmlReader{builder: builder, starts: map[string]func(){htmlTags{}.lineBreak(): builder.newline}, ends: map[string]func(){}}
	reader.handlers = map[html.TokenType]func(html.Token){
		html.TextToken:           reader.text,
		html.StartTagToken:       reader.start,
		html.EndTagToken:         reader.end,
		html.SelfClosingTagToken: reader.selfClosing,
	}
	reader.addTags(htmlTags{}.dropped(), reader.enterDropped, reader.leaveDropped)
	reader.addBoundaries(newHtmlReader(builder, nil).rules)
	return reader
}

func (reader *flatHtmlReader) addBoundaries(rules map[string]elementRule) {
	for tag, rule := range rules {
		reader.addBoundary(tag, rule.block)
	}
}

func (reader *flatHtmlReader) addBoundary(tag string, block bool) {
	if !block {
		return
	}
	reader.addTags([]string{tag}, reader.boundary, reader.boundary)
}

func (reader *flatHtmlReader) addTags(tags []string, start func(), end func()) {
	for _, tag := range tags {
		reader.starts[tag] = start
		reader.ends[tag] = end
	}
}

func (reader *flatHtmlReader) read(source string) {
	tokenizer := html.NewTokenizer(strings.NewReader(source))
	for tokenizer.Next() != html.ErrorToken {
		reader.handlerFor(tokenizer.Token())
	}
}

func (reader *flatHtmlReader) handlerFor(token html.Token) {
	handler, known := reader.handlers[token.Type]
	if !known {
		return
	}
	handler(token)
}

func (reader *flatHtmlReader) text(token html.Token) {
	if reader.dropped > 0 {
		return
	}
	reader.builder.addFlowing(token.Data, Span{})
}

func (reader *flatHtmlReader) start(token html.Token) {
	reader.run(reader.starts, token.Data)
}

func (reader *flatHtmlReader) end(token html.Token) {
	reader.run(reader.ends, token.Data)
}

func (reader *flatHtmlReader) selfClosing(token html.Token) {
	reader.start(token)
	reader.end(token)
}

func (reader *flatHtmlReader) run(actions map[string]func(), tag string) {
	action, known := actions[tag]
	if !known {
		return
	}
	action()
}

func (reader *flatHtmlReader) enterDropped() {
	reader.dropped++
}

func (reader *flatHtmlReader) leaveDropped() {
	reader.dropped = max(reader.dropped-1, 0)
}

func (reader *flatHtmlReader) boundary() {
	reader.builder.setShape(Block{Kind: Kinds{}.Paragraph()})
}
