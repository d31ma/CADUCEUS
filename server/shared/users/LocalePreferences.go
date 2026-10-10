// LocalePreferences keeps the interface locale identifier.
type LocalePreferences struct{}

// InterfaceLocale answers the stored interface locale, or "auto".
func (LocalePreferences) InterfaceLocale(value any) string {
	locales := []string{"auto", "en", "de", "es", "es-419", "fr", "ko", "pl", "ja", "zh-CN",
		"zh-TW", "pt", "pt-BR", "gl", "tr", "ru", "cs", "uk", "hu", "he", "ar"}
	return PreferenceChoices{}.Text(value, locales, "auto")
}
