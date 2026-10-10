// GrammarResult is the answer of one check.
type GrammarResult struct {
	Language         string         `json:"language"`
	DetectedLanguage string         `json:"detectedLanguage"`
	Matches          []GrammarMatch `json:"matches"`
}
