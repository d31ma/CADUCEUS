package core

type Block struct {
	Kind    string    `json:"kind"`
	Level   int       `json:"level"`
	Ordered bool      `json:"ordered"`
	Spans   []Span    `json:"spans"`
	Image   *ImageRef `json:"image,omitempty"`
	Cells   []string  `json:"cells"`
}
