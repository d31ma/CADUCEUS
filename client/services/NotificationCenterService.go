import (
	"encoding/json"
	"slices"
)

// NotificationCenterService derives the shell's notices (offline, update
// ready, reconnecting mailboxes, disabled rules, push prompt), keeps the
// user's dismissals on this device and runs the notice actions.
// @Service
type NotificationCenterService struct {
	// @Needs
	rulesClient *RulesClient
	// @Needs
	notificationsClient *NotificationsClient
	// @Needs
	notifications             Notifications
	applicationEventPublisher ApplicationEventPublisher
	Open                      bool
	Visible                   []Notice
	Highest                   Notice
	HasHighest                bool
	Count                     int
	ButtonLabel               string
	state                     NoticeState
	dismissed                 []string
	stored                    map[string]string
	email                     string
	token                     string
	features                  BrowserFeatures
}

// @OnStart
func (center *NotificationCenterService) Start() {
	center.state = NoticeState{Online: true, AccountErrors: []string{}}
	center.dismissed = []string{}
	center.stored = map[string]string{}
	center.refresh()
}

// HearConnectivity shows the offline notice while the browser is offline.
// @OnEvent({"replay":true})
func (center *NotificationCenterService) HearConnectivity(changed ConnectivityChanged) {
	center.state.Online = changed.Online
	center.refresh()
}

// HearFeatures learns whether this browser can offer push.
// @OnEvent({"replay":true})
func (center *NotificationCenterService) HearFeatures(features BrowserFeatures) {
	center.features = features
}

// HearStorage keeps the stored dismissals until the user is known.
// @OnEvent({"replay":true})
func (center *NotificationCenterService) HearStorage(stored StoredValues) {
	center.stored = stored.Local
	center.restoreDismissals()
}

// HearSession restores this user's dismissals and loads the rule and push
// sources.
// @OnEvent({"replay":true})
func (center *NotificationCenterService) HearSession(changed SessionChanged) {
	center.email = changed.Email
	center.token = changed.Token
	center.restoreDismissals()
	FlowUtils{}.When(changed.SignedIn, center.loadSources)()
}

// HearUpdate shows the update notice once a new worker is installed.
// @OnEvent
func (center *NotificationCenterService) HearUpdate(ready UpdateReady) {
	center.state.UpdateVersion = ready.Version
	center.refresh()
}

// HearStream shows the reconnecting notice while the live mail stream fails.
// @OnEvent
func (center *NotificationCenterService) HearStream(health MailStreamHealth) {
	center.state.AccountErrors = center.failingMailboxes(health.Healthy)
	center.refresh()
}

// HearEscape closes the panel.
// @OnEvent
func (center *NotificationCenterService) HearEscape(pressed EscapePressed) {
	FlowUtils{}.When(center.Open, center.Close)()
}

// Toggle opens or closes the panel.
func (center *NotificationCenterService) Toggle() {
	FlowUtils{}.Either(center.Open, center.Close, center.openPanel)()
}

// Close closes the panel and returns the focus to where it was.
func (center *NotificationCenterService) Close() {
	center.Open = false
	center.applicationEventPublisher.PublishEvent(FocusRequested{Restore: true}, false)
}

// Dismiss hides an optional notice for this user on this device.
func (center *NotificationCenterService) Dismiss(id string) {
	FlowUtils{}.When(NoticeUtils{}.Dismissable(center.Visible, id), func() { center.dismiss(id) })()
}

// RunAction runs one notice action and closes the panel.
func (center *NotificationCenterService) RunAction(action string) {
	center.action(action)()
	center.Close()
}

func (center *NotificationCenterService) openPanel() {
	center.Open = true
	center.applicationEventPublisher.PublishEvent(FocusRequested{Selector: ".app-notification-panel > header button", Remember: true}, false)
}

func (center *NotificationCenterService) loadSources() {
	rules, _ := center.rulesClient.All(center.bearer())
	subscriptions, failure := center.notificationsClient.Subscriptions(center.bearer())
	permission, _ := center.notifications.Permission()
	center.state.DisabledRuleCount = center.disabledRules(rules)
	center.state.PushPrompt = NoticeUtils{}.PushPrompt(center.features, permission, failure == nil, len(subscriptions))
	center.refresh()
}

func (center *NotificationCenterService) refresh() {
	center.Visible = NoticeUtils{}.Visible(NoticeUtils{}.Build(center.state), center.dismissed)
	center.Count = len(center.Visible)
	center.HasHighest = center.Count > 0
	center.Highest = center.first(center.Visible)
	center.ButtonLabel = NoticeUtils{}.Label(center.Count)
}

func (center *NotificationCenterService) restoreDismissals() {
	center.dismissed = []string{}
	json.Unmarshal([]byte(center.stored[center.dismissalKey()]), &center.dismissed)
	center.dismissed = center.lastHundred(center.dismissed)
	center.refresh()
}

func (center *NotificationCenterService) dismiss(id string) {
	center.dismissed = center.lastHundred(append(slices.DeleteFunc(slices.Clone(center.dismissed), func(value string) bool { return value == id }), id))
	encoded, _ := json.Marshal(center.dismissed)
	center.applicationEventPublisher.PublishEvent(StorageWritten{Area: "local", Values: map[string]string{center.dismissalKey(): string(encoded)}, Removed: []string{}}, false)
	center.refresh()
}

func (center *NotificationCenterService) action(action string) func() {
	actions := map[string]func(){
		"try-now":               center.retry,
		"retry-accounts":        center.retry,
		"reload-update":         func() { center.publish(NavigationRequested{Reload: true}) },
		"manage-accounts":       func() { center.publish(NavigationRequested{Target: "/settings#mailboxes"}) },
		"view-rules":            func() { center.publish(NavigationRequested{Target: "/settings#mail-rules"}) },
		"notification-settings": func() { center.publish(NavigationRequested{Target: "/settings#notifications"}) },
	}
	if chosen, found := actions[action]; found {
		return chosen
	}
	return func() {}
}

func (center *NotificationCenterService) retry() {
	center.state.AccountErrors = []string{}
	center.refresh()
	center.publish(InboxRefreshRequested{Reason: "retry"})
}

func (center *NotificationCenterService) publish(value any) {
	center.applicationEventPublisher.PublishEvent(value, false)
}

func (center *NotificationCenterService) failingMailboxes(healthy bool) []string {
	if healthy {
		return []string{}
	}
	return []string{center.email}
}

func (center *NotificationCenterService) disabledRules(rules []RuleSummary) int {
	return len(slices.DeleteFunc(slices.Clone(rules), center.enabledRule))
}

// enabledRule is true unless the rule says enabled: false.
func (center *NotificationCenterService) enabledRule(rule RuleSummary) bool {
	return rule.Enabled == nil || *rule.Enabled
}

func (center *NotificationCenterService) first(notices []Notice) Notice {
	if len(notices) == 0 {
		return Notice{Actions: []NoticeAction{}}
	}
	return notices[0]
}

// lastHundred keeps the newest hundred dismissals.
func (center *NotificationCenterService) lastHundred(values []string) []string {
	return values[max(0, len(values)-center.kept()):]
}

func (NotificationCenterService) kept() int { return 100 }

func (center *NotificationCenterService) bearer() string {
	return "Bearer " + center.token
}

func (center *NotificationCenterService) dismissalKey() string {
	return "caduceus-notification-dismissals:" + TextUtils{}.FirstOf(center.email, "anonymous")
}
