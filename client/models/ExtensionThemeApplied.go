// ExtensionThemeApplied carries the CSS variables of the enabled extension
// theme to the extension bridge; empty means no extension theme.
type ExtensionThemeApplied struct {
	Variables map[string]string `json:"variables"`
}
