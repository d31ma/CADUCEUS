import "errors"

// PushNotificationsService turns new-mail Web Push alerts on or off for this
// device. PushSubscriptionService.js asks the browser for permission and
// holds the push subscription; this service registers it with the server.
// @Service
type PushNotificationsService struct {
	// @Needs
	notificationsClient       *NotificationsClient
	applicationEventPublisher ApplicationEventPublisher
	timeZoneOffset            float64
	authorization             string
	Supported                 bool
	Permission                string
	Subscriptions             []PushSubscriptionSummary
	Loading                   bool
	Error                     string
}

// @OnStart
func (service *PushNotificationsService) Start() {
	service.Permission = "default"
	service.Subscriptions = []PushSubscriptionSummary{}
}

// @OnEvent({"replay":true})
func (service *PushNotificationsService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
	refused := false
	FlowUtils{}.When(changed.SignedIn, func() { refused = service.refresh() })()
	service.announce(refused)
}

// @OnEvent({"replay":true})
func (service *PushNotificationsService) Place(platform DevicePlatform) {
	service.timeZoneOffset = platform.TimeZoneOffset
}

// @OnEvent({"replay":true})
func (service *PushNotificationsService) Learn(support PushSupport) {
	service.Supported = support.Supported
	service.Permission = support.Permission
}

func (service *PushNotificationsService) Enable() {
	service.Error = ""
	FlowUtils{}.Either(service.Supported, func() {
		service.Loading = true
		service.applicationEventPublisher.PublishEvent(PushPermissionRequested{Reason: "enable"}, false)
	}, func() { service.Error = "This browser cannot receive Web Push notifications." })()
}

// @OnEvent
func (service *PushNotificationsService) Permitted(answer PushPermissionAnswered) {
	service.Permission = answer.Permission
	FlowUtils{}.Either(answer.Permission == service.granted(), service.subscribe, func() {
		service.Error = "Notifications were not allowed."
		service.Loading = false
	})()
	service.announce(false)
}

// @OnEvent
func (service *PushNotificationsService) Subscribed(created PushSubscriptionCreated) {
	_, failure := service.save(created)
	FlowUtils{}.Either(failure == nil, func() {
		service.refresh()
		service.toast("Notifications enabled.")
	}, func() { service.Error = service.enableFailure() })()
	service.Loading = false
	service.announce(false)
}

func (service *PushNotificationsService) Disable() {
	service.Error = ""
	FlowUtils{}.When(service.Supported, func() {
		service.Loading = true
		service.applicationEventPublisher.PublishEvent(PushUnsubscribeRequested{Reason: "disable"}, false)
	})()
}

// @OnEvent
func (service *PushNotificationsService) Unsubscribed(removed PushSubscriptionRemoved) {
	FlowUtils{}.Either(removed.Problem == "", func() {
		FlowUtils{}.When(removed.Endpoint != "", func() { service.forget(removed.Endpoint) })()
		service.refresh()
		service.toast("Notifications disabled.")
	}, func() { service.Error = "Unable to disable notifications on this device." })()
	service.Loading = false
	service.announce(false)
}

func (service *PushNotificationsService) StatusLabel() string {
	return service.status(service.active())
}

func (service *PushNotificationsService) HelpText() string {
	return service.help(service.active())
}

func (service *PushNotificationsService) Rows() []PushSubscriptionRow {
	return service.rows()
}

func (service *PushNotificationsService) subscribe() {
	key, failure := service.notificationsClient.VapidKey(service.authorization)
	FlowUtils{}.Either(failure == nil, func() {
		service.applicationEventPublisher.PublishEvent(PushSubscribeRequested{PublicKey: key.PublicKey}, false)
	}, func() {
		service.Error = service.enableFailure()
		service.Loading = false
	})()
}

func (service *PushNotificationsService) save(created PushSubscriptionCreated) (BehaviorValue, error) {
	if created.Problem != "" {
		return nil, service.notificationsFailure()
	}
	return service.notificationsClient.Subscribe(service.authorization, created.Subscription)
}

func (service *PushNotificationsService) forget(endpoint string) {
	_, _ = service.notificationsClient.Unsubscribe(service.authorization, map[string]any{"endpoint": endpoint})
}

// refresh reads this account's push subscriptions and says whether the
// server refused the read.
func (service *PushNotificationsService) refresh() bool {
	subscriptions, failure := service.notificationsClient.Subscriptions(service.authorization)
	service.Subscriptions = append([]PushSubscriptionSummary{}, subscriptions...)
	return HTTPUtils{}.Refused(failure)
}

func (service *PushNotificationsService) active() bool {
	return ChoiceUtils{}.All(service.Permission == service.granted(), len(service.Subscriptions) > 0)
}

func (service *PushNotificationsService) status(active bool) string {
	switch {
	case !service.Supported:
		return "Not available on this browser"
	case active:
		return "On"
	case service.Permission == service.denied():
		return "Blocked"
	}
	return "Off"
}

func (service *PushNotificationsService) help(active bool) string {
	switch {
	case !service.Supported:
		return "Install Caduceus with a browser that supports Web Push to receive new mail alerts."
	case service.Permission == service.denied():
		return "Notifications are blocked in this browser. Allow them in site settings to turn alerts on."
	case active:
		return "This device will alert you when new mail arrives."
	}
	return "Turn this on for an installed desktop or mobile app feel when mail arrives."
}

func (service *PushNotificationsService) rows() []PushSubscriptionRow {
	rows := []PushSubscriptionRow{}
	for _, subscription := range service.Subscriptions {
		rows = append(rows, PushSubscriptionRow{UpdatedAtLabel: DateLabelUtils{}.StoredDate(subscription.UpdatedAt, service.timeZoneOffset)})
	}
	return rows
}

func (service *PushNotificationsService) enableFailure() string {
	return "Unable to enable notifications on this device."
}

func (service *PushNotificationsService) notificationsFailure() error {
	return errors.New(service.enableFailure())
}

func (service *PushNotificationsService) toast(message string) {
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (service *PushNotificationsService) announce(refused bool) {
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "push-notifications", Refused: refused}, false)
}

func (service *PushNotificationsService) granted() string { return "granted" }

func (service *PushNotificationsService) denied() string { return "denied" }
