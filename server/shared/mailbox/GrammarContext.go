// GrammarContext is the text around one issue.
type GrammarContext struct {
	Text   string `json:"text"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}
