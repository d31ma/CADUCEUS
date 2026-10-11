import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
)

// AppearanceUtils keeps the bounded theme, font and mail-layout catalogs and
// turns a choice into the attributes of the <html> element. Only catalog
// identifiers reach the document: the font stacks, accent colours and scales
// themselves live in shared/assets/themes.css, keyed by these attributes.
type AppearanceUtils struct{}

// Themes keeps a known theme choice and falls back to the defaults, the
// first entry of each catalog.
func (AppearanceUtils) Themes(chosen ThemePreferences) ThemePreferences {
	catalog := AppearanceUtils{}.catalog()
	return ThemePreferences{
		ActiveTheme: AppearanceUtils{}.known(chosen.ActiveTheme, catalog.Themes),
		LightTheme:  AppearanceUtils{}.known(chosen.LightTheme, catalog.LightThemes),
		DarkTheme:   AppearanceUtils{}.known(chosen.DarkTheme, catalog.DarkThemes),
	}
}

// StoredThemes reads the theme choice kept on this device, migrating the
// older single-value "caduceus-theme" preference once.
func (AppearanceUtils) StoredThemes(text string, legacy string) ThemePreferences {
	chosen := AppearanceUtils{}.legacyThemes(legacy)
	json.Unmarshal([]byte(text), &chosen)
	return AppearanceUtils{}.Themes(chosen)
}

// Resolved is the theme in use: the active one, or for "automatic" the light
// or dark theme that matches the device.
func (AppearanceUtils) Resolved(themes ThemePreferences, systemDark bool) string {
	return AppearanceUtils{}.resolved(themes.ActiveTheme, AppearanceUtils{}.pick(systemDark, themes.DarkTheme, themes.LightTheme))
}

// BaseMode is "dark" for the dark catalog themes and "light" otherwise.
func (AppearanceUtils) BaseMode(theme string) string {
	modes := AppearanceUtils{}.modes()
	return AppearanceUtils{}.pick(slices.Contains(AppearanceUtils{}.catalog().DarkThemes, theme), modes.Dark, modes.Light)
}

// Opposite is the other base mode.
func (AppearanceUtils) Opposite(base string) string {
	modes := AppearanceUtils{}.modes()
	return AppearanceUtils{}.pick(base == modes.Dark, modes.Light, modes.Dark)
}

// InterfaceFont keeps a catalog interface font.
func (AppearanceUtils) InterfaceFont(font string) string {
	return AppearanceUtils{}.known(font, AppearanceUtils{}.catalog().Fonts)
}

// ComposerFont keeps a catalog composer font; the first matches the interface.
func (AppearanceUtils) ComposerFont(font string) string {
	return AppearanceUtils{}.known(font, append([]string{"interface"}, AppearanceUtils{}.catalog().Fonts...))
}

// Accent keeps a catalog accent colour.
func (AppearanceUtils) Accent(accent string) string {
	return AppearanceUtils{}.known(accent, AppearanceUtils{}.catalog().Accents)
}

// Mail reads mail preferences over the defaults and keeps the catalog values.
func (AppearanceUtils) Mail(text string) MailPreferences {
	preferences := AppearanceUtils{}.DefaultMail()
	json.Unmarshal([]byte(text), &preferences)
	return AppearanceUtils{}.mailCatalog(preferences)
}

// ChosenMail keeps the catalog values of mail preferences that came in a model.
func (AppearanceUtils) ChosenMail(chosen *MailPreferences) MailPreferences {
	text, _ := json.Marshal(chosen)
	return AppearanceUtils{}.Mail(string(text))
}

// DefaultMail is the mail-client preference model before the user changes it.
func (AppearanceUtils) DefaultMail() MailPreferences {
	catalog := AppearanceUtils{}.catalog()
	return MailPreferences{
		WorkspaceLayout: catalog.Layouts[0], InterfaceScale: 1, MarkAsReadDelay: 500, BackspaceAction: catalog.Backspace[0], SwipeEnabled: true,
		ConversationOrder: catalog.Orders[0], EmailRenderMode: catalog.RenderModes[0], RichText: true, Spellcheck: true, DefaultSendAction: catalog.SendActions[0],
		DefaultReplyAction: catalog.ReplyActions[0], UndoSendDelay: 5000, AutoCcBySender: map[string][]string{}, AutoBccBySender: map[string][]string{},
		DisplayAttachmentPreviews: true, UnsnoozeToTop: true, FindInMailDisabledDomains: []string{},
	}
}

// ThemeAttributes are the <html> attributes of a theme choice.
func (AppearanceUtils) ThemeAttributes(themes ThemePreferences, resolved string) map[string]string {
	base := AppearanceUtils{}.BaseMode(resolved)
	return map[string]string{"w-theme": base, "data-theme": base, "data-ui-theme": resolved, "data-theme-choice": themes.ActiveTheme}
}

// MailAttributes are the <html> attributes of the mail layout preferences.
func (AppearanceUtils) MailAttributes(mail MailPreferences) map[string]string {
	return map[string]string{
		"data-mail-layout":         mail.WorkspaceLayout,
		"data-message-width":       AppearanceUtils{}.pick(mail.RestrictMessageWidth, "restricted", "fluid"),
		"data-message-theme":       mail.EmailRenderMode,
		"data-conversation-order":  mail.ConversationOrder,
		"data-attachment-previews": AppearanceUtils{}.enabled(mail.DisplayAttachmentPreviews),
		"data-rich-composer":       AppearanceUtils{}.enabled(mail.RichText),
		"data-mail-swipe":          AppearanceUtils{}.enabled(mail.SwipeEnabled),
		"data-interface-scale":     strconv.FormatFloat(mail.InterfaceScale, 'f', -1, 64),
	}
}

// Merged copies the attributes of two groups into one table.
func (AppearanceUtils) Merged(first map[string]string, second map[string]string) map[string]string {
	return AppearanceUtils{}.into(AppearanceUtils{}.into(map[string]string{}, first), second)
}

func (AppearanceUtils) into(target map[string]string, source map[string]string) map[string]string {
	for name, value := range source {
		target[name] = value
	}
	return target
}

func (AppearanceUtils) legacyThemes(legacy string) ThemePreferences {
	modes := AppearanceUtils{}.modes()
	return ThemePreferences{ActiveTheme: map[string]string{modes.Light: modes.Light, modes.Dark: modes.Dark, "auto": modes.Automatic}[legacy]}
}

func (AppearanceUtils) resolved(active string, automatic string) string {
	if active == (AppearanceUtils{}).modes().Automatic {
		return automatic
	}
	return active
}

func (AppearanceUtils) mailCatalog(preferences MailPreferences) MailPreferences {
	catalog := AppearanceUtils{}.catalog()
	preferences.WorkspaceLayout = AppearanceUtils{}.known(preferences.WorkspaceLayout, catalog.Layouts)
	preferences.ConversationOrder = AppearanceUtils{}.known(preferences.ConversationOrder, catalog.Orders)
	preferences.EmailRenderMode = AppearanceUtils{}.known(preferences.EmailRenderMode, catalog.RenderModes)
	preferences.BackspaceAction = AppearanceUtils{}.known(preferences.BackspaceAction, catalog.Backspace)
	preferences.DefaultSendAction = AppearanceUtils{}.known(preferences.DefaultSendAction, catalog.SendActions)
	preferences.DefaultReplyAction = AppearanceUtils{}.known(preferences.DefaultReplyAction, catalog.ReplyActions)
	preferences.InterfaceScale = AppearanceUtils{}.scale(preferences.InterfaceScale, catalog.Scales)
	preferences.DefaultFrom = strings.ToLower(strings.TrimSpace(preferences.DefaultFrom))
	return preferences
}

func (AppearanceUtils) scale(value float64, scales []float64) float64 {
	if slices.Contains(scales, value) {
		return value
	}
	return 1
}

// known keeps a catalog value and falls back to the catalog's first entry.
func (AppearanceUtils) known(value string, catalog []string) string {
	if slices.Contains(catalog, value) {
		return value
	}
	return catalog[0]
}

func (AppearanceUtils) enabled(yes bool) string {
	return AppearanceUtils{}.pick(yes, "enabled", "disabled")
}

func (AppearanceUtils) pick(yes bool, chosen string, other string) string {
	if yes {
		return chosen
	}
	return other
}

// catalog lists every bounded appearance choice; the first entry of each
// list is its default.
func (AppearanceUtils) catalog() AppearanceCatalog {
	modes := AppearanceUtils{}.modes()
	lights := []string{modes.Light, "less-is-more", "taiga", "ubuntu"}
	darks := []string{modes.Dark, "darkside"}
	return AppearanceCatalog{
		Themes: append(append([]string{modes.Automatic}, lights...), darks...), LightThemes: lights, DarkThemes: darks,
		Fonts:        []string{"plex-sans", "system", "humanist", "serif", "monospace"},
		Accents:      []string{"teal", "blue", "purple", "pink", "orange", "green", "red", "slate"},
		Layouts:      []string{"split", "list", "split-vertical"},
		Orders:       []string{"ascending", "descending"},
		RenderModes:  []string{"theme", modes.Light, modes.Dark},
		Backspace:    []string{"archive", "trash"},
		SendActions:  []string{"send", "send-and-archive"},
		ReplyActions: []string{"reply", "reply-all"},
		Scales:       []float64{1, 0.8, 0.9, 1.1, 1.2},
	}
}

func (AppearanceUtils) modes() ThemeModes {
	return ThemeModes{Automatic: "automatic", Light: "light", Dark: "dark"}
}
