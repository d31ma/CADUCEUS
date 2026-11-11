// @RequestRoute("/api/v1/notify/settings")
// @SignedIn
type SettingsController struct {
	// @Needs
	preferences *PreferencesService
}

// @GetRoute
func (controller *SettingsController) GetSettings(
	// @FromHeader("authorization")
	authorization string,
) (NotifySettings, error) {
	return controller.preferences.Get(authorization)
}

// @PutRoute
func (controller *SettingsController) PutSettings(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	settings NotifySettings,
) (NotifySettings, error) {
	return controller.preferences.Put(authorization, settings)
}
