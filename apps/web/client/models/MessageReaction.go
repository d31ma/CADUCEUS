type MessageReaction struct {
	Emoji string `json:"emoji"`
	Glyph string `json:"glyph"`
	Count int    `json:"count"`
	Mine  bool   `json:"mine"`
	Label string `json:"label"`
}
