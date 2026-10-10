import "cmp"

// FontPreferenceUtils is the Go form of shared/scripts/font-preferences.js.
// Only local font stacks exist; a preference stores an identifier, never CSS.
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

func (utils FontPreferenceUtils) NormalizeInterface(value any) string {
	return cmp.Or(utils.knownIds(utils.InterfaceOptions())[utils.text(value)], utils.defaultInterface())
}

func (utils FontPreferenceUtils) NormalizeComposer(value any) string {
	return cmp.Or(utils.knownIds(utils.ComposerOptions())[utils.text(value)], utils.defaultComposer())
}

// InterfacePreview is the inline style of the interface font preview line.
func (utils FontPreferenceUtils) InterfacePreview(id string) string {
	return utils.preview(utils.stackOf(utils.InterfaceOptions(), utils.NormalizeInterface(id)))
}

func (utils FontPreferenceUtils) ComposerPreview(id string) string {
	return utils.preview(utils.stackOf(utils.ComposerOptions(), utils.NormalizeComposer(id)))
}

func (FontPreferenceUtils) preview(stack string) string { return "font-family:" + stack }

func (FontPreferenceUtils) defaultInterface() string { return "plex-sans" }

func (FontPreferenceUtils) defaultComposer() string { return "interface" }

func (FontPreferenceUtils) text(value any) string {
	text, _ := value.(string)
	return text
}

// stacks maps each identifier to itself, so an unknown one reads as empty.
func (FontPreferenceUtils) knownIds(options []FontOption) map[string]string {
	known := map[string]string{}
	for _, option := range options {
		known[option.ID] = option.ID
	}
	return known
}

func (FontPreferenceUtils) stackOf(options []FontOption, id string) string {
	stacks := map[string]string{}
	for _, option := range options {
		stacks[option.ID] = option.Stack
	}
	return stacks[id]
}
