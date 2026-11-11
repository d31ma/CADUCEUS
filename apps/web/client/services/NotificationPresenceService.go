import (
	"maps"
	"slices"
	"strings"
)

// @Service
type NotificationPresenceService struct {
	MyState           string
	DoNotDisturbUntil string
	MenuOpen          bool
	Choices           []PresenceChoice
	Problem           string
	LoadProblem       string
	chosen            string
	known             map[string]string
	knownDomain       string
	watched           []string
	// @Needs
	api *NotificationPresenceApiService
	// @Needs
	access *NotificationTokenService
	// @Needs
	times *NotificationTimeService
	// @Needs
	pageVisibility            PageVisibility
	applicationEventPublisher ApplicationEventPublisher
}

// @OnStart
func (service *NotificationPresenceService) Start() {
	service.MyState = service.active()
	service.applicationEventPublisher.PublishEvent(PresenceAnnounceRequested{}, false)
}

// @OnEvent
// @InBackground
func (service *NotificationPresenceService) Announce(requested PresenceAnnounceRequested) {
	visible, failure := service.pageVisibility.Visible()
	service.refresh(service.automaticState(visible, failure))
}

// @OnEvent
// @InBackground
func (service *NotificationPresenceService) Shown(changed VisibilityChanged) {
	service.refresh(service.automaticState(changed.Visible, nil))
	service.reloadWhenVisible(changed.Visible)
}

// @OnEvent
// @InBackground
func (service *NotificationPresenceService) Load(requested PresenceWatchRequested) {
	service.forgetOtherDomain()
	service.watched = service.unionOf(requested.MemberIds)
	presences, problem := service.api.Find(requested.MemberIds)
	service.known = service.merged(presences)
	service.LoadProblem = problem
}

func (service *NotificationPresenceService) stateOf(memberId string) string {
	return service.stateOrOffline(service.known[memberId])
}

func (service *NotificationPresenceService) labelOf(memberId string) string {
	return service.labels()[service.stateOf(memberId)]
}

func (service *NotificationPresenceService) Quiet() bool {
	return service.quiet()
}

func (service *NotificationPresenceService) ToggleMenu() {
	service.MenuOpen = !service.MenuOpen
	service.expireWhenPassed()
	service.Problem = ""
	service.Choices = service.choicesNow()
}

func (service *NotificationPresenceService) CloseMenu() {
	service.MenuOpen = false
}

func (service *NotificationPresenceService) DoNotDisturb(name string) {
	until := service.untilFor(name)
	problem := service.api.Announce(PresenceRequest{State: service.doNotDisturb(), DoNotDisturbUntil: until})
	service.keepWhenSaved(problem, name, until)
	service.finish(problem)
}

func (service *NotificationPresenceService) SetActive() {
	problem := service.api.Announce(PresenceRequest{State: service.active()})
	service.clearWhenSaved(problem)
	service.finish(problem)
}

func (service *NotificationPresenceService) Name() string {
	account := service.access.Account()
	return service.nameOr(account.DisplayName, account.Address)
}

func (service *NotificationPresenceService) Initials() string {
	account := service.access.Account()
	return service.initialsOf(strings.Fields(account.DisplayName + " " + account.Address))
}

func (service *NotificationPresenceService) StatusText() string {
	return service.statusTexts()[service.MyState]
}

func (service *NotificationPresenceService) ButtonLabel() string {
	return service.Name() + ", " + service.StatusText() + ". Open status menu"
}

func (service *NotificationPresenceService) refresh(state string) {
	service.expireWhenPassed()
	service.Problem = service.sendUnlessQuiet(state)
}

func (service *NotificationPresenceService) finish(problem string) {
	service.Problem = problem
	service.MenuOpen = problem != ""
	service.Choices = service.choicesNow()
}

func (service *NotificationPresenceService) expireWhenPassed() {
	if service.MyState != service.doNotDisturb() || !service.times.Passed(service.DoNotDisturbUntil) {
		return
	}
	service.MyState = service.active()
	service.DoNotDisturbUntil = ""
	service.chosen = ""
}

func (service *NotificationPresenceService) sendUnlessQuiet(state string) string {
	if service.MyState == service.doNotDisturb() {
		return service.Problem
	}
	service.MyState = state
	return service.api.Announce(PresenceRequest{State: state})
}

func (service *NotificationPresenceService) automaticState(visible bool, failure error) string {
	if failure == nil && !visible {
		return service.idle()
	}
	return service.active()
}

func (service *NotificationPresenceService) reloadWhenVisible(visible bool) {
	if !visible {
		return
	}
	service.Load(PresenceWatchRequested{MemberIds: service.watched})
}

func (service *NotificationPresenceService) keepWhenSaved(problem string, name string, until string) {
	if problem != "" {
		return
	}
	service.MyState = service.doNotDisturb()
	service.DoNotDisturbUntil = until
	service.chosen = name
}

func (service *NotificationPresenceService) clearWhenSaved(problem string) {
	if problem != "" {
		return
	}
	service.MyState = service.active()
	service.DoNotDisturbUntil = ""
	service.chosen = ""
}

func (service *NotificationPresenceService) quiet() bool {
	return service.MyState == service.doNotDisturb() && !service.times.Passed(service.DoNotDisturbUntil)
}

func (service *NotificationPresenceService) forgetOtherDomain() {
	if service.knownDomain == service.access.DomainId() {
		return
	}
	service.known = map[string]string{}
	service.knownDomain = service.access.DomainId()
}

func (service *NotificationPresenceService) merged(presences []Presence) map[string]string {
	known := map[string]string{}
	maps.Copy(known, service.known)
	for _, presence := range presences {
		known[presence.MemberId] = presence.State
	}
	return known
}

func (service *NotificationPresenceService) unionOf(memberIds []string) []string {
	union := append(slices.Clone(service.watched), memberIds...)
	slices.Sort(union)
	return slices.Compact(union)
}

func (service *NotificationPresenceService) stateOrOffline(state string) string {
	if state == "" {
		return service.offline()
	}
	return state
}

func (service *NotificationPresenceService) nameOr(name string, address string) string {
	if name == "" {
		return address
	}
	return name
}

func (service *NotificationPresenceService) initialsOf(words []string) string {
	initials := ""
	for _, word := range words[:min(len(words), service.initialCount())] {
		initials += strings.ToUpper(string([]rune(word)[0]))
	}
	return initials
}

func (service *NotificationPresenceService) untilFor(name string) string {
	minutes, found := service.choiceMinutes()[name]
	if !found {
		return service.times.TomorrowMorning()
	}
	return service.times.Later(minutes)
}

func (service *NotificationPresenceService) choicesNow() []PresenceChoice {
	choices := []PresenceChoice{}
	for _, choice := range service.choiceTable() {
		choices = append(choices, service.choiceOf(choice))
	}
	return choices
}

func (service *NotificationPresenceService) choiceOf(choice PresenceChoice) PresenceChoice {
	choice.EndText = "Until " + service.times.EndText(service.untilFor(choice.Name))
	choice.Selected = service.isChosen(choice.Name)
	return choice
}

func (service *NotificationPresenceService) isChosen(name string) bool {
	return service.MyState == service.doNotDisturb() && service.chosen == name
}

func (service *NotificationPresenceService) statusTexts() map[string]string {
	return map[string]string{
		service.active():       "Active",
		service.idle():         "Idle",
		service.doNotDisturb(): "Do not disturb until " + service.times.EndText(service.DoNotDisturbUntil),
	}
}

func (service *NotificationPresenceService) labels() map[string]string {
	return map[string]string{
		service.active():       ", active",
		service.idle():         ", idle",
		service.doNotDisturb(): ", do not disturb",
		service.offline():      ", offline",
	}
}

func (service *NotificationPresenceService) choiceTable() []PresenceChoice {
	return []PresenceChoice{
		{Name: service.thirtyMinutes(), Label: "For 30 minutes"},
		{Name: service.oneHour(), Label: "For 1 hour"},
		{Name: service.twoHours(), Label: "For 2 hours"},
		{Name: service.tomorrow(), Label: "Until tomorrow"},
	}
}

func (service *NotificationPresenceService) choiceMinutes() map[string]int {
	return map[string]int{service.thirtyMinutes(): 30, service.oneHour(): 60, service.twoHours(): 120}
}

func (service *NotificationPresenceService) initialCount() int {
	return 2
}

func (service *NotificationPresenceService) thirtyMinutes() string {
	return "30m"
}

func (service *NotificationPresenceService) oneHour() string {
	return "1h"
}

func (service *NotificationPresenceService) twoHours() string {
	return "2h"
}

func (service *NotificationPresenceService) idle() string {
	return "idle"
}

func (service *NotificationPresenceService) tomorrow() string {
	return "tomorrow"
}

func (service *NotificationPresenceService) active() string {
	return "active"
}

func (service *NotificationPresenceService) doNotDisturb() string {
	return "dnd"
}

func (service *NotificationPresenceService) offline() string {
	return "offline"
}
