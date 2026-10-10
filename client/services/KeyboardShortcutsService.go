import (
	"cmp"
	"strings"
)

// KeyboardShortcutsService edits the account's keyboard shortcut overrides
// against the command catalog that the view passes in from
// shared/data/keymap.json. The page's key listener applies saved overrides.
// @Service
// @NeedsAll
type KeyboardShortcutsService struct {
	// @Required
	Keymap Keymap
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	apple                     bool
	draft                     map[string]string
	Loading                   bool
	Saving                    bool
	Error                     string
	Status                    string
}

// @OnStart
func (service *KeyboardShortcutsService) Start() {
	service.Loading = true
	service.draft = service.defaults()
}

// @OnEvent({"replay":true})
func (service *KeyboardShortcutsService) Open(session AccountSession) {
	service.authorization = "Bearer " + session.Token
}

// @OnEvent({"replay":true})
func (service *KeyboardShortcutsService) Learn(platform DevicePlatform) {
	service.apple = platform.Apple
}

// @OnEvent({"replay":true})
func (service *KeyboardShortcutsService) Load(preferences UserPreferences) {
	overrides := map[string]string{}
	PreferenceUtils{}.Decode(preferences.Values[service.key()], &overrides)
	service.show(overrides)
	service.Loading = false
}

func (service *KeyboardShortcutsService) Groups() []ShortcutGroup {
	return KeyboardShortcutUtils{}.Groups(service.catalog(), service.draft, service.apple)
}

// Capture records one edited field; an empty field disables the command.
func (service *KeyboardShortcutsService) Capture(id string, value string) {
	service.draft[id] = cmp.Or(strings.TrimSpace(value), KeyboardShortcutUtils{}.Disabled())
	service.Error = ""
	service.Status = ""
}

func (service *KeyboardShortcutsService) Save() error {
	FlowUtils{}.When(!service.Saving, service.save)
	return nil
}

func (service *KeyboardShortcutsService) Reset() error {
	service.draft = service.defaults()
	service.Save()
	return nil
}

func (service *KeyboardShortcutsService) save() {
	validation := KeyboardShortcutUtils{}.Validate(service.catalog(), service.draft, service.apple)
	service.Error = validation.Problem
	service.Status = ""
	FlowUtils{}.When(validation.Valid, func() { service.send(validation.Overrides) })
}

func (service *KeyboardShortcutsService) send(overrides map[string]string) {
	service.Saving = true
	stored, failure := service.usersClient.SavePreferences(service.authorization, map[string]any{service.key(): overrides})
	service.Saving = false
	service.showSaved(PreferenceUtils{}.Object(stored), failure)
}

func (service *KeyboardShortcutsService) showSaved(stored map[string]any, failure error) {
	if failure != nil {
		service.Error = "Could not save keyboard shortcuts."
		return
	}
	overrides := map[string]string{}
	PreferenceUtils{}.Decode(stored[service.key()], &overrides)
	service.show(overrides)
	service.Status = "Keyboard shortcuts saved."
	service.applicationEventPublisher.PublishEvent(KeyboardShortcutOverrides{Overrides: overrides}, false)
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: service.Status}, false)
}

// show starts the editor from the stored overrides, as the keyboard runtime
// does: overrides that do not validate are dropped as a whole.
func (service *KeyboardShortcutsService) show(overrides map[string]string) {
	validation := KeyboardShortcutUtils{}.Validate(service.catalog(), overrides, service.apple)
	applied := map[bool]map[string]string{true: validation.Overrides, false: {}}[validation.Valid]
	service.draft = service.defaults()
	for id, binding := range applied {
		service.draft[id] = binding
	}
}

func (service *KeyboardShortcutsService) defaults() map[string]string {
	defaults := map[string]string{}
	for _, action := range service.catalog() {
		defaults[action.ID] = action.Binding
	}
	return defaults
}

// catalog is the command catalog with each default in its canonical form.
func (service *KeyboardShortcutsService) catalog() []KeymapAction {
	catalog := []KeymapAction{}
	for _, action := range service.Keymap.Actions {
		action.Binding = KeyboardShortcutUtils{}.NormalizeBinding(action.Binding)
		action.Group = cmp.Or(action.Group, "Other")
		action.Label = cmp.Or(action.Label, action.ID)
		catalog = append(catalog, action)
	}
	return catalog
}

func (service *KeyboardShortcutsService) key() string { return "keyboardShortcutOverrides" }
