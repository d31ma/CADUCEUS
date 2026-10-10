// SettingsService holds the one "still loading" flag of the settings view.
// Each section publishes SettingsActivity when its first load is done; the
// event also makes the view render after work that finished in the background.
// @Service
type SettingsService struct {
	finished map[string]bool
	Loading  bool
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
}

func (settings *SettingsService) missing() int {
	missing := 0
	for _, section := range []string{"aliases", "preferences", "connected-accounts", "folder-roles", "domain-routes", "inbox-rules", "passkeys", "push-notifications", "signatures", "templates", "mcp", "extensions", "users"} {
		missing += map[bool]int{true: 0, false: 1}[settings.finished[section]]
	}
	return missing
}
