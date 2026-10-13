// TranslationDetected is the language the browser found in the open message,
// and the saved preference for that language.
type TranslationDetected struct {
	MessageId  string  `json:"messageId"`
	Language   string  `json:"language"`
	Confidence float64 `json:"confidence"`
	Preference string  `json:"preference"`
}
