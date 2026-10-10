// FontPreferences keeps the interface and composer font identifiers.
type FontPreferences struct{}

// InterfaceFont answers the stored interface font, or the default.
func (FontPreferences) InterfaceFont(value any) string {
	return PreferenceChoices{}.Text(value, []string{"plex-sans", "system", "humanist", "serif", "monospace"}, "plex-sans")
}

// ComposerFont answers the stored composer font, or the default.
func (FontPreferences) ComposerFont(value any) string {
	return PreferenceChoices{}.Text(value, []string{"interface", "plex-sans", "system", "humanist", "serif", "monospace"}, "interface")
}
