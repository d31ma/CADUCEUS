package richtext

import "strings"

type Block struct {
	Kind    string    `json:"kind"`
	Level   int       `json:"level"`
	Ordered bool      `json:"ordered"`
	Spans   []Span    `json:"spans"`
	Image   *ImageRef `json:"image,omitempty"`
	Cells   []string  `json:"cells"`
}

func (block Block) plainText() string {
	return block.spanText() + strings.Join(block.Cells, "\t") + block.altText()
}

func (block Block) spanText() string {
	var text strings.Builder
	for _, span := range block.Spans {
		text.WriteString(span.Text)
	}
	return text.String()
}

func (block Block) altText() string {
	if block.Image == nil {
		return ""
	}
	return block.Image.Alt
}
