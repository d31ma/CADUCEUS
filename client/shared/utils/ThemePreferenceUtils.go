import (
	"slices"
	"strings"
)

// ThemePreferenceUtils is the Go form of the catalog and normalization in
// shared/scripts/theme-preferences.js. Applying a theme to the document is
// the page's job (see DocumentAppearanceService.js).
type ThemePreferenceUtils struct{}

func (utils ThemePreferenceUtils) Options() []ThemeOption {
	light, dark, automatic := utils.light(), utils.dark(), utils.automatic()
	page, accent, ink := "#f8faf9", "#1f6f8b", "#1c2224"
	return []ThemeOption{
		{ID: automatic, Label: "Automatic", Description: "Follow this device’s light or dark appearance.", BaseMode: automatic, Colors: []string{page, accent, ink}},
		{ID: light, Label: "Light", Description: "CADUCEUS’s balanced light interface.", BaseMode: light, Colors: []string{page, "#eef3f1", accent}},
		{ID: dark, Label: "Dark", Description: "CADUCEUS’s low-glare dark interface.", BaseMode: dark, Colors: []string{ink, "#161c1e", "#83cde3"}},
		{ID: "darkside", Label: "Darkside", Description: "A deep aubergine sidebar with a warm accent.", BaseMode: dark, Colors: []string{"#242230", "#313042", "#f18260"}},
		{ID: "less-is-more", Label: "Less Is More", Description: "A quiet, compact, low-chrome workspace.", BaseMode: light, Colors: []string{"#ffffff", "#f7f7f5", "#606b73"}},
		{ID: "taiga", Label: "Taiga", Description: "A clean, cool, mailbox-inspired workspace.", BaseMode: light, Colors: []string{"#f5f8fa", "#e8eef2", "#4f7c8f"}},
		{ID: "ubuntu", Label: "Ubuntu", Description: "Aubergine navigation with Ubuntu orange.", BaseMode: light, Colors: []string{"#f7f4f2", "#4b243f", "#e95420"}},
	}
}

func (utils ThemePreferenceUtils) LightOptions() []ThemeOption {
	return slices.DeleteFunc(utils.Options(), func(option ThemeOption) bool { return option.BaseMode != utils.light() })
}

func (utils ThemePreferenceUtils) DarkOptions() []ThemeOption {
	return slices.DeleteFunc(utils.Options(), func(option ThemeOption) bool { return option.BaseMode != utils.dark() })
}

func (utils ThemePreferenceUtils) Normalize(value any) ThemePreferences {
	input := PreferenceUtils{}.Object(value)
	return ThemePreferences{
		ActiveTheme: PreferenceUtils{}.Choice(PreferenceUtils{}.Text(input, "activeTheme"), utils.ids(utils.Options()), utils.automatic()),
		LightTheme:  PreferenceUtils{}.Choice(PreferenceUtils{}.Text(input, "lightTheme"), utils.ids(utils.LightOptions()), utils.light()),
		DarkTheme:   PreferenceUtils{}.Choice(PreferenceUtils{}.Text(input, "darkTheme"), utils.ids(utils.DarkOptions()), utils.dark()),
	}
}

// PreviewStyle is the inline style that paints one theme's three swatches.
func (ThemePreferenceUtils) PreviewStyle(theme ThemeOption) string {
	colors := append(append([]string{}, theme.Colors...), "#fff", "#eee", "#777")
	return strings.Join([]string{"--theme-preview-1:" + colors[0], "--theme-preview-2:" + colors[1], "--theme-preview-3:" + colors[2]}, ";")
}

func (ThemePreferenceUtils) automatic() string { return "automatic" }

func (ThemePreferenceUtils) light() string { return "light" }

func (ThemePreferenceUtils) dark() string { return "dark" }

func (ThemePreferenceUtils) ids(options []ThemeOption) []string {
	ids := []string{}
	for _, option := range options {
		ids = append(ids, option.ID)
	}
	return ids
}
