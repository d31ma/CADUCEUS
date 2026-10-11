import "cmp"

// CalendarSharingService chooses how much of the calendar same-domain
// teammates may see: private, busy only, or full details.
// @Service
type CalendarSharingService struct {
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	Visibility                string
}

// @OnStart
func (service *CalendarSharingService) Start() {
	service.Visibility = "private"
}

// @OnEvent({"replay":true})
func (service *CalendarSharingService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
}

// @OnEvent({"replay":true})
func (service *CalendarSharingService) Load(preferences UserPreferences) {
	service.Visibility = cmp.Or(preferences.CalendarVisibility, service.Visibility)
}

func (service *CalendarSharingService) Choose(visibility string) error {
	service.Visibility = visibility
	_, failure := service.usersClient.SavePreferences(service.authorization, map[string]any{"calendarVisibility": visibility})
	message := HTTPUtils{}.Message(failure, "Could not save preference.", "Network error.")
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: ChoiceUtils{}.Text(failure == nil, "Calendar sharing updated.", message)}, false)
	return nil
}
