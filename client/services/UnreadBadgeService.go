import "slices"

// UnreadBadgeService chooses whether app and notification badges count every
// hosted account or one of them.
// @Service
type UnreadBadgeService struct {
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	domains                   []string
	names                     map[string]string
	Scope                     string
	Domain                    string
	Saving                    bool
	Error                     string
}

// @OnStart
func (service *UnreadBadgeService) Start() {
	service.Scope = service.everyAccount()
}

// @OnEvent({"replay":true})
func (service *UnreadBadgeService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
	service.domains = MailboxUtils{}.Hosted(changed.Domains)
}

// @OnEvent({"replay":true})
func (service *UnreadBadgeService) Load(preferences UserPreferences) {
	service.names = preferences.MailboxNames
	service.show(service.normalized(preferences))
}

func (service *UnreadBadgeService) DomainOptions() []DomainOption {
	return service.options()
}

func (service *UnreadBadgeService) SetScope(scope string) error {
	first := PreferenceUtils{}.Joined(service.domains[:min(1, len(service.domains))])
	known := slices.Contains([]string{service.everyAccount(), service.oneAccount()}, scope)
	FlowUtils{}.When(known, func() { service.save(scope, ChoiceUtils{}.Text(service.Domain != "", service.Domain, first)) })()
	return nil
}

func (service *UnreadBadgeService) SetDomain(domain string) error {
	FlowUtils{}.When(slices.Contains(service.domains, domain), func() { service.save(service.oneAccount(), domain) })()
	return nil
}

func (service *UnreadBadgeService) save(scope string, domain string) {
	next := UnreadBadgeUtils{}.Normalize(scope, domain, service.domains)
	refused := ChoiceUtils{}.All(scope == service.oneAccount(), next.Scope != service.oneAccount())
	FlowUtils{}.Either(refused, func() { service.Error = "Choose an account assigned to your profile." }, func() {
		FlowUtils{}.When(!service.Saving, func() { service.send(next) })()
	})()
}

func (service *UnreadBadgeService) send(next BadgePreference) {
	previous := BadgePreference{Scope: service.Scope, Domain: service.Domain}
	service.show(next)
	service.Saving = true
	service.Error = ""
	stored, failure := service.usersClient.SavePreferences(service.authorization, map[string]any{"unreadBadgeScope": next.Scope, "unreadBadgeDomain": next.Domain})
	service.Saving = false
	FlowUtils{}.Either(failure == nil, func() { service.saved(stored) }, func() {
		service.show(previous)
		service.Error = HTTPUtils{}.Message(failure, "Could not save unread badge preference.", "Network error.")
	})()
}

func (service *UnreadBadgeService) saved(stored UserPreferences) {
	service.show(service.normalized(stored))
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Unread badge preference saved."}, false)
}

func (service *UnreadBadgeService) normalized(preferences UserPreferences) BadgePreference {
	return UnreadBadgeUtils{}.Normalize(preferences.UnreadBadgeScope, preferences.UnreadBadgeDomain, service.domains)
}

func (service *UnreadBadgeService) show(preference BadgePreference) {
	service.Scope = preference.Scope
	service.Domain = preference.Domain
}

func (service *UnreadBadgeService) options() []DomainOption {
	options := []DomainOption{}
	for _, domain := range service.domains {
		options = append(options, DomainOption{Domain: domain, Label: MailboxUtils{}.Label(service.names, domain)})
	}
	return options
}

func (service *UnreadBadgeService) everyAccount() string { return "all" }

func (service *UnreadBadgeService) oneAccount() string { return "domain" }
