import (
	"encoding/json"
	"errors"
	"time"
)

// @TestSetup
type NotificationTestSetup struct{}

func (setup NotificationTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	configuration := NewBehaviorConfiguration()
	configuration.Http["notify"] = setup
	configuration.Http["NotifyClient"] = setup
	configuration.Http["presence"] = setup
	configuration.Http["PresenceClient"] = setup
	return configuration, nil
}

func (setup NotificationTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	if setup.bearer(request) != "Bearer access-1" {
		return nil, errors.New("unauthenticated")
	}
	switch request.Method + " " + request.Path {
	case "GET /api/v1/notify/notifications":
		return setup.page(setup.text(request.Query["cursor"]))
	case "PUT /api/v1/notify/notifications/n1/read", "PUT /api/v1/notify/notifications/n2/read", "PUT /api/v1/notify/notifications/n4/read":
		return nil, nil
	case "GET /api/v1/notify/settings":
		return NotificationSettings{EmailSummaryAfterMinutes: 30, MutedConversationIds: []string{"c-9"}}, nil
	case "PUT /api/v1/notify/settings":
		return setup.settingsOf(request), nil
	case "PUT /api/v1/realtime/presence":
		return nil, nil
	case "GET /api/v1/realtime/presence":
		return setup.presences(setup.text(request.Query["memberIds"]))
	}
	return nil, errors.New("no fake answer for " + request.Method + " " + request.Path)
}

func (setup NotificationTestSetup) page(cursor string) (BehaviorValue, error) {
	switch cursor {
	case "":
		return NotificationPage{Items: setup.firstItems(), NextCursor: "page-2"}, nil
	case "page-2":
		return NotificationPage{Items: []Notification{setup.item("n5", "mention", "2026-11-03T08:00:00Z", "2026-11-03T09:00:00Z")}}, nil
	}
	return nil, errors.New("unknown cursor")
}

func (setup NotificationTestSetup) firstItems() []Notification {
	return []Notification{
		setup.item("n1", "mention", "2026-11-10T10:05:00Z", ""),
		setup.item("n2", "dm", "2026-11-10T09:58:00Z", ""),
		setup.item("n3", "thread_reply", "2026-11-10T09:31:00Z", "2026-11-10T09:40:00Z"),
		setup.item("n4", "email", "2026-11-09T12:10:00Z", ""),
	}
}

func (setup NotificationTestSetup) item(notificationId string, kind string, createdAt string, readAt string) Notification {
	return Notification{NotificationId: notificationId, Kind: kind, ConversationId: "c-" + notificationId, MessageId: "m-" + notificationId, CreatedAt: createdAt, ReadAt: readAt}
}

func (setup NotificationTestSetup) presences(memberIds string) (BehaviorValue, error) {
	switch memberIds {
	case "m-2,m-3,m-x":
		return []Presence{{MemberId: "m-2", State: "active", LastSeenAt: "2026-11-10T10:59:00Z"}, {MemberId: "m-3", State: "dnd", DoNotDisturbUntil: "2026-11-10T12:00:00Z", LastSeenAt: "2026-11-10T10:00:00Z"}}, nil
	case "m-4":
		return []Presence{{MemberId: "m-4", State: "idle", LastSeenAt: "2026-11-10T10:00:00Z"}}, nil
	}
	return []Presence{}, nil
}

func (setup NotificationTestSetup) settingsOf(request *BehaviorHttpRequest) NotificationSettings {
	text, _ := json.Marshal(request.Body)
	settings := NotificationSettings{}
	_ = json.Unmarshal(text, &settings)
	return settings
}

func (setup NotificationTestSetup) bearer(request *BehaviorHttpRequest) string {
	return setup.text(request.Headers["Authorization"]) + setup.text(request.Headers["authorization"])
}

func (setup NotificationTestSetup) text(value any) string {
	text, _ := value.(string)
	return text
}

func (setup NotificationTestSetup) answerBrowser() {
	BrowserFakes{}.Answer("clock", "now", 1794308400000)
	BrowserFakes{}.Answer("pageVisibility", "visible", true)
	BrowserFakes{}.Answer("notifications", "permission", "granted")
	BrowserFakes{}.Answer("notifications", "requestPermission", true)
	BrowserFakes{}.Answer("notifications", "show", true)
}

func (setup NotificationTestSetup) access(domainId string) *NotificationTokenService {
	accounts := &SessionAccountService{}
	accounts.Add(Me{MemberId: "m-1", DomainId: domainId, DisplayName: "Ana Ruiz", PrimaryAddress: "ana@" + domainId})
	return setup.accessFor(accounts)
}

func (setup NotificationTestSetup) signedOutAccess() *NotificationTokenService {
	return setup.accessFor(&SessionAccountService{})
}

func (setup NotificationTestSetup) accessFor(accounts *SessionAccountService) *NotificationTokenService {
	tokens := &SessionTokenService{tokens: map[string]SessionTokens{
		"example.com":    {AccessToken: "access-1", ExpiresAtMs: 9000000000000},
		"broken.example": {AccessToken: "access-broken", ExpiresAtMs: 9000000000000},
	}}
	setup.answerBrowser()
	return &NotificationTokenService{tokens: tokens, accounts: accounts, problems: &NotificationProblemService{}}
}

func (setup NotificationTestSetup) times() *NotificationTimeService {
	return &NotificationTimeService{zone: time.UTC}
}

func (setup NotificationTestSetup) api(behaviors *BehaviorContext, access *NotificationTokenService) *NotificationApiService {
	return &NotificationApiService{notify: NewNotifyClientWith(behaviors), access: access, problems: &NotificationProblemService{}}
}

func (setup NotificationTestSetup) presenceApi(behaviors *BehaviorContext, access *NotificationTokenService) *NotificationPresenceApiService {
	return &NotificationPresenceApiService{presence: NewPresenceClientWith(behaviors), access: access, problems: &NotificationProblemService{}}
}

func (setup NotificationTestSetup) presence(behaviors *BehaviorContext, access *NotificationTokenService) *NotificationPresenceService {
	return &NotificationPresenceService{api: setup.presenceApi(behaviors, access), access: access, times: setup.times(), MyState: "active"}
}

func (setup NotificationTestSetup) push(behaviors *BehaviorContext, access *NotificationTokenService) *NotificationPushService {
	return &NotificationPushService{api: setup.api(behaviors, access), presence: setup.presence(behaviors, access), kinds: &NotificationKindService{}}
}

func (setup NotificationTestSetup) rows() *NotificationRowService {
	return &NotificationRowService{times: setup.times(), kinds: &NotificationKindService{}}
}

func (setup NotificationTestSetup) list(behaviors *BehaviorContext, access *NotificationTokenService) *NotificationService {
	return &NotificationService{api: setup.api(behaviors, access), rows: setup.rows(), push: setup.push(behaviors, access)}
}
