// TranslationChoices are the languages the message reader translates
// without asking and the languages it never offers to translate.
type TranslationChoices struct {
	Automatic []string `json:"automatic"`
	Disabled  []string `json:"disabled"`
}
