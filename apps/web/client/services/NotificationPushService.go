import (
	"errors"
	"strconv"
)

// @Service
type NotificationPushService struct {
	Permission string
	Panel      string
	// @LocalState
	DeviceOff      bool
	SummaryMinutes int
	SummaryChoices []NotificationSummaryChoice
	SettingsStatus string
	SaveProblem    string
	Saved          bool
	Problem        string
	TestSent       bool
	muted          []string
	shown          map[string]bool
	// @Needs
	notifications Notifications
	// @Needs
	api *NotificationApiService
	// @Needs
	presence *NotificationPresenceService
	// @Needs
	kinds                     *NotificationKindService
	applicationEventPublisher ApplicationEventPublisher
}

// @OnStart
func (service *NotificationPushService) Start() {
	service.SettingsStatus = service.statusLoading()
	service.SummaryChoices = service.choices()
	service.applicationEventPublisher.PublishEvent(NotificationSettingsLoadRequested{}, false)
}

// @OnEvent
// @InBackground
func (service *NotificationPushService) LoadSettings(requested NotificationSettingsLoadRequested) {
	service.readPermission()
	settings, problem := service.api.Settings()
	service.SettingsStatus = service.statusOf(problem)
	service.SummaryMinutes = settings.EmailSummaryAfterMinutes
	service.muted = settings.MutedConversationIds
	service.SummaryChoices = service.choices()
}

// @OnEvent
// @InBackground
func (service *NotificationPushService) DomainChanged(changed SessionDomainChanged) {
	service.SettingsStatus = service.statusLoading()
	service.Saved = false
	service.SaveProblem = ""
	service.LoadSettings(NotificationSettingsLoadRequested{})
}

// @OnEvent
func (service *NotificationPushService) Shown(changed VisibilityChanged) {
	service.readPermission()
}

// @OnEvent
func (service *NotificationPushService) PermissionChanged(changed NotificationPermissionChanged) {
	service.Permission = changed.Permission
	service.Panel = service.panelOf()
}

// @OnEvent
func (service *NotificationPushService) CallFailed(failed BrowserCallFailed) {
	service.readPermission()
	service.Problem = service.notAllowed()
}

func (service *NotificationPushService) RetrySettings() {
	service.SettingsStatus = service.statusLoading()
	service.applicationEventPublisher.PublishEvent(NotificationSettingsLoadRequested{}, false)
}

func (service *NotificationPushService) TurnOn() {
	service.DeviceOff = false
	service.Problem = service.problemOf(service.requestWhenNotAsked(), service.notAllowed())
	service.Panel = service.panelOf()
}

func (service *NotificationPushService) TurnOff() {
	service.DeviceOff = true
	service.Panel = service.panelOf()
}

func (service *NotificationPushService) CheckAgain() {
	service.readPermission()
	service.Problem = ""
}

func (service *NotificationPushService) SendTest() {
	failure := service.notifications.Show(service.title(), "Notifications are on for this browser.")
	service.TestSent = failure == nil
	service.Problem = service.problemOf(failure, service.notShown())
}

func (service *NotificationPushService) ChooseSummary(minutes int) {
	service.saveWhenChanged(minutes)
}

func (service *NotificationPushService) saveWhenChanged(minutes int) {
	if minutes == service.SummaryMinutes {
		return
	}
	service.save(minutes)
}

func (service *NotificationPushService) save(minutes int) {
	request := NotificationSettings{EmailSummaryAfterMinutes: minutes, MutedConversationIds: append([]string{}, service.muted...)}
	saved, problem := service.api.SaveSettings(request)
	service.SummaryMinutes = service.minutesAfter(saved, problem)
	service.SaveProblem = problem
	service.Saved = problem == ""
	service.SummaryChoices = service.choices()
}

func (service *NotificationPushService) Announce(items []Notification, hidden bool) {
	service.readPermission()
	failure := service.showEach(service.allowed(items, hidden, service.presence.Quiet()))
	service.Problem = service.problemOf(failure, service.notShown())
}

func (service *NotificationPushService) readPermission() {
	permission, failure := service.notifications.Permission()
	service.Permission = service.permissionOr(permission, failure)
	service.Panel = service.panelOf()
}

func (service *NotificationPushService) permissionOr(permission string, failure error) string {
	if failure != nil {
		return service.unsupported()
	}
	return permission
}

func (service *NotificationPushService) panelOf() string {
	return service.offWhenTurnedOff(service.namedPanel(service.Permission))
}

func (service *NotificationPushService) namedPanel(permission string) string {
	panels := map[string]string{service.permissionNotAsked(): "notAsked", "granted": service.panelOn(), "denied": "blocked"}
	panel, found := panels[permission]
	if found {
		return panel
	}
	return service.unsupported()
}

func (service *NotificationPushService) unsupported() string {
	return "unsupported"
}

func (service *NotificationPushService) offWhenTurnedOff(panel string) string {
	if !service.DeviceOff || panel != service.panelOn() {
		return panel
	}
	return "off"
}

func (service *NotificationPushService) requestWhenNotAsked() error {
	if service.Permission != service.permissionNotAsked() {
		return nil
	}
	return service.notifications.RequestPermission()
}

func (service *NotificationPushService) allowed(items []Notification, hidden bool, quiet bool) []Notification {
	if !hidden || quiet || service.Panel != service.panelOn() {
		return []Notification{}
	}
	return items
}

func (service *NotificationPushService) showEach(items []Notification) error {
	service.shown = service.shownMap()
	var failure error
	for _, item := range items {
		failure = errors.Join(failure, service.showOne(item))
	}
	return failure
}

func (service *NotificationPushService) showOne(item Notification) error {
	if service.shown[item.NotificationId] {
		return nil
	}
	service.shown[item.NotificationId] = true
	return service.notifications.Show(service.title(), service.kinds.Text(item.Kind))
}

func (service *NotificationPushService) shownMap() map[string]bool {
	if service.shown == nil {
		return map[string]bool{}
	}
	return service.shown
}

func (service *NotificationPushService) problemOf(failure error, problem string) string {
	if failure == nil {
		return ""
	}
	return problem
}

func (service *NotificationPushService) minutesAfter(saved NotificationSettings, problem string) int {
	if problem != "" {
		return service.SummaryMinutes
	}
	return saved.EmailSummaryAfterMinutes
}

func (service *NotificationPushService) statusOf(problem string) string {
	if problem != "" {
		return problem
	}
	return "ready"
}

func (service *NotificationPushService) choices() []NotificationSummaryChoice {
	choices := []NotificationSummaryChoice{}
	for _, choice := range service.summaryTable() {
		choices = append(choices, service.choiceOf(choice))
	}
	return choices
}

func (service *NotificationPushService) choiceOf(choice NotificationSummaryChoice) NotificationSummaryChoice {
	choice.Id = "summary-" + strconv.Itoa(choice.Minutes)
	choice.Selected = choice.Minutes == service.SummaryMinutes
	return choice
}

func (service *NotificationPushService) summaryTable() []NotificationSummaryChoice {
	return []NotificationSummaryChoice{
		{Minutes: 0, Label: "Off"},
		{Minutes: 15, Label: "15 minutes"},
		{Minutes: 30, Label: "30 minutes"},
		{Minutes: 60, Label: "1 hour"},
		{Minutes: 240, Label: "4 hours"},
		{Minutes: 1440, Label: "24 hours"},
	}
}

func (service *NotificationPushService) title() string {
	return "Mesail"
}

func (service *NotificationPushService) permissionNotAsked() string {
	return "default"
}

func (service *NotificationPushService) panelOn() string {
	return "on"
}

func (service *NotificationPushService) notAllowed() string {
	return "notAllowed"
}

func (service *NotificationPushService) notShown() string {
	return "notShown"
}

func (service *NotificationPushService) statusLoading() string {
	return "loading"
}
