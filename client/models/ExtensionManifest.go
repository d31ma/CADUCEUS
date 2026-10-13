// ExtensionManifest is the validated manifest of an installed extension:
// what it asks for, what a plugin adds to the page, and a theme's tokens.
type ExtensionManifest struct {
	Description   string                  `json:"description"`
	Permissions   []string                `json:"permissions"`
	Contributions []ExtensionContribution `json:"contributions"`
	Theme         ExtensionTheme          `json:"theme"`
}
