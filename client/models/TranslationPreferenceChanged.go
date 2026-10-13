// TranslationPreferenceChanged is the saved preference for the detected
// language: offer, always or never. Translate asks for a translation now and
// Original shows the original text.
type TranslationPreferenceChanged struct {
	Preference string `json:"preference"`
	Translate  bool   `json:"translate"`
	Original   bool   `json:"original"`
}
