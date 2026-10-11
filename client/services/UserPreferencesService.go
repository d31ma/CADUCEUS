// UserPreferencesService reads the account preferences once and hands them
// to every settings section as one retained UserPreferences event. The
// settings view names it (userPreferences.loaded) so that Tac creates it. Each
// section saves its own patch through its UsersClient and publishes the
// stored answer the same way, so the other sections follow.
// @Service
type UserPreferencesService struct {
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	Loaded                    bool
}

// @OnEvent({"replay":true})
func (service *UserPreferencesService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
	FlowUtils{}.When(changed.SignedIn, service.load)()
}

func (service *UserPreferencesService) load() {
	loaded, failure := service.usersClient.Preferences(service.authorization)
	FlowUtils{}.When(failure == nil, func() { service.applicationEventPublisher.PublishEvent(loaded, true) })()
	service.Loaded = true
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "preferences", Refused: HTTPUtils{}.Refused(failure)}, false)
}
