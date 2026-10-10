// UserPreferencesService reads the account preferences once and hands them
// to every settings section as one retained UserPreferences event. Each
// section saves its own patch through its UsersClient.
// @Service
type UserPreferencesService struct {
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
}

// @OnEvent({"replay":true})
func (service *UserPreferencesService) Open(session AccountSession) error {
	values, _ := service.usersClient.Preferences("Bearer " + session.Token)
	service.applicationEventPublisher.PublishEvent(UserPreferences{Values: PreferenceUtils{}.Object(values)}, true)
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "preferences"}, false)
	return nil
}
