// @Service
type SettingsService struct {
	busy    map[string]bool
	Loading bool
}

// @OnStart
func (settings *SettingsService) Start() {
	settings.busy = map[string]bool{}
	settings.Loading = true
}

// @OnEvent
func (settings *SettingsService) Track(activity SettingsActivity) {
	settings.mark(activity)
	settings.Loading = len(settings.busy) > 0
}

func (settings *SettingsService) mark(activity SettingsActivity) {
	if activity.Busy {
		settings.busy[activity.Section] = true
		return
	}
	delete(settings.busy, activity.Section)
}
