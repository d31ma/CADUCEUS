package fakes

import "mesail.dev/packages/core"

type FakeRenderer struct {
	Failure error
}

func (renderer *FakeRenderer) Markdown(body string) (core.RichText, error) {
	block := core.Block{Kind: "paragraph", Spans: []core.Span{{Text: body}}, Cells: []string{}}
	return core.RichText{Blocks: []core.Block{block}}, renderer.Failure
}
