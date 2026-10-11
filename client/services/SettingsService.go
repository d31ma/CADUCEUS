// SettingsService holds the one "still loading" flag of the settings view.
// Each section publishes SettingsActivity when its first load is done; the
// event also makes the view render after work that finished in the
// background. When the server refused a section's read, it asks
// SessionService once whether the session still holds.
// @Service
type SettingsService struct {
	applicationEventPublisher ApplicationEventPublisher
	finished                  map[string]bool
	checked                   bool
	Loading                   bool
}

// @OnStart
func (settings *SettingsService) Start() {
	settings.finished = map[string]bool{}
	settings.Loading = true
}

// @OnEvent
func (settings *SettingsService) Track(activity SettingsActivity) {
	settings.finished[activity.Section] = true
	settings.Loading = settings.missing() > 0
	FlowUtils{}.When(ChoiceUtils{}.All(activity.Refused, !settings.checked), settings.check)()
}

func (settings *SettingsService) check() {
	settings.checked = true
	settings.applicationEventPublisher.PublishEvent(SessionCheckRequested{Reason: "settings"}, false)
}

func (settings *SettingsService) missing() int {
	missing := 0
	for _, section := range []string{"aliases", "preferences", "connected-accounts", "folder-roles", "domain-routes", "inbox-rules", "passkeys", "push-notifications", "signatures", "templates", "mcp", "extensions", "users"} {
		missing += map[bool]int{true: 0, false: 1}[settings.finished[section]]
	}
	return missing
}
