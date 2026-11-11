package adapters

import (
	"encoding/json"

	"mesail.dev/packages/core"
	"mesail.dev/packages/richtext"
)

type RichTextRenderer struct{}

func (renderer RichTextRenderer) Markdown(body string) (core.RichText, error) {
	result := richtext.Converter{}.FromMarkdown(body)
	blocks, err := renderer.blocks(result.Blocks)
	return core.RichText{Blocks: blocks, Truncated: result.Truncated}, err
}

func (RichTextRenderer) blocks(source []richtext.Block) ([]core.Block, error) {
	blocks := []core.Block{}
	encoded, err := json.Marshal(source)
	if err != nil {
		return blocks, err
	}
	return blocks, json.Unmarshal(encoded, &blocks)
}
