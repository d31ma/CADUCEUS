// FontPreferenceUtils describes the local font catalog for the settings
// view. A preference stores an identifier, never CSS; AppearanceUtils keeps
// the identifiers and shared/assets/themes.css holds the stacks.
type FontPreferenceUtils struct{}

func (FontPreferenceUtils) InterfaceOptions() []FontOption {
	return []FontOption{
		{ID: "plex-sans", Label: "IBM Plex Sans", Stack: `"IBM Plex Sans", Inter, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`},
		{ID: "system", Label: "System UI", Stack: `system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`},
		{ID: "humanist", Label: "Humanist Sans", Stack: `"Avenir Next", Avenir, Candara, "Segoe UI", sans-serif`},
		{ID: "serif", Label: "Readable Serif", Stack: `Charter, "Bitstream Charter", "Sitka Text", Cambria, Georgia, serif`},
		{ID: "monospace", Label: "Monospace", Stack: `"IBM Plex Mono", "SFMono-Regular", Consolas, "Liberation Mono", monospace`},
	}
}

func (utils FontPreferenceUtils) ComposerOptions() []FontOption {
	return append([]FontOption{{ID: "interface", Label: "Match interface", Stack: "var(--caduceus-interface-font)"}}, utils.InterfaceOptions()...)
}

// NormalizeInterface keeps a catalog interface font and the default otherwise.
func (FontPreferenceUtils) NormalizeInterface(font string) string {
	return AppearanceUtils{}.InterfaceFont(font)
}

// NormalizeComposer keeps a catalog composer font and the default otherwise.
func (FontPreferenceUtils) NormalizeComposer(font string) string {
	return AppearanceUtils{}.ComposerFont(font)
}

// InterfacePreview is the inline style of the interface font preview line.
func (utils FontPreferenceUtils) InterfacePreview(id string) string {
	return utils.preview(utils.stackOf(utils.InterfaceOptions(), utils.NormalizeInterface(id)))
}

func (utils FontPreferenceUtils) ComposerPreview(id string) string {
	return utils.preview(utils.stackOf(utils.ComposerOptions(), utils.NormalizeComposer(id)))
}

func (FontPreferenceUtils) preview(stack string) string { return "font-family:" + stack }

func (FontPreferenceUtils) stackOf(options []FontOption, id string) string {
	stacks := map[string]string{}
	for _, option := range options {
		stacks[option.ID] = option.Stack
	}
	return stacks[id]
}
