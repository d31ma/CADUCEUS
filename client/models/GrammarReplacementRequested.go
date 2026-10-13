type GrammarReplacementRequested struct {
	Offset      int    `json:"offset"`
	Length      int    `json:"length"`
	Replacement string `json:"replacement"`
}
