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
func (service *CalendarSharingService) Open(session AccountSession) {
	service.authorization = "Bearer " + session.Token
}

// @OnEvent({"replay":true})
func (service *CalendarSharingService) Load(preferences UserPreferences) {
	service.Visibility = cmp.Or(PreferenceUtils{}.Text(preferences.Values, service.key()), service.Visibility)
}

func (service *CalendarSharingService) Choose(visibility string) error {
	service.Visibility = visibility
	_, failure := service.usersClient.SavePreferences(service.authorization, map[string]any{service.key(): visibility})
	message := FlowUtils{}.Text(failure == nil, "Calendar sharing updated.", "Could not save preference.")
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
	return nil
}

func (service *CalendarSharingService) key() string { return "calendarVisibility" }
