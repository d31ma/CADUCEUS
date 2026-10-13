import "strings"

// LocalePreferences keeps the interface locale identifier.
type LocalePreferences struct{}

// InterfaceLocale answers the stored interface locale, or "auto".
func (preferences LocalePreferences) InterfaceLocale(value any) string {
	return PreferenceChoices{}.Text(value, preferences.locales(), "auto")
}

// Checked answers the interface locale a caller sent, or why it is refused.
func (preferences LocalePreferences) Checked(value any) (string, *PreferenceProblem) {
	text, isText := value.(string)
	if !isText || !(PreferenceChoices{}).Includes(preferences.locales(), text) {
		return "", &PreferenceProblem{Message: "interfaceLocale must be one of: " + strings.Join(preferences.locales(), ", ")}
	}
	return text, nil
}

func (LocalePreferences) locales() []string {
	return []string{"auto", "en", "de", "es", "es-419", "fr", "ko", "pl", "ja", "zh-CN",
		"zh-TW", "pt", "pt-BR", "gl", "tr", "ru", "cs", "uk", "hu", "he", "ar"}
}
