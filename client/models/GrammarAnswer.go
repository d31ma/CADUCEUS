type GrammarAnswer struct {
	Matches          []GrammarMatch `json:"matches"`
	DetectedLanguage string         `json:"detectedLanguage"`
}
