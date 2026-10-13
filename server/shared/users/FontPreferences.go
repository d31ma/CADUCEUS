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

// IsInterfaceFont reports whether value is an interface font identifier.
func (fonts FontPreferences) IsInterfaceFont(value any) bool {
	text, isText := value.(string)
	return isText && fonts.InterfaceFont(value) == text
}

// IsComposerFont reports whether value is a composer font identifier.
func (fonts FontPreferences) IsComposerFont(value any) bool {
	text, isText := value.(string)
	return isText && fonts.ComposerFont(value) == text
}
