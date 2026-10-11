import "strings"

// NavigationService highlights the active item of the app-nav component: Compose,
// Settings, or the Inbox for every other route.
// @Service
type NavigationService struct {
	ActiveView string
	locale     string
}

// @OnStart
func (navigation *NavigationService) Start() {
	navigation.ActiveView = "inbox"
	navigation.locale = "en"
}

// HearLocation follows the current route.
// @OnEvent({"replay":true})
func (navigation *NavigationService) HearLocation(place PageLocation) {
	navigation.ActiveView = navigation.view(place.Path)
}

// HearStorage follows the interface locale kept on this device.
// @OnEvent({"replay":true})
func (navigation *NavigationService) HearStorage(stored StoredValues) {
	navigation.locale = LocaleUtils{}.Resolve(stored.Local["caduceus-interface-locale-v1"], "")
}

// Text translates one fixed interface message in the active locale.
func (navigation *NavigationService) Text(id string) string {
	return LocaleUtils{}.Translate(navigation.locale, id, nil)
}

func (NavigationService) view(path string) string {
	switch {
	case strings.HasPrefix(path, "/compose"):
		return "compose"
	case strings.HasPrefix(path, "/settings"):
		return "settings"
	}
	return "inbox"
}
