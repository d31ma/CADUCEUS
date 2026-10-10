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
func (service *UnreadBadgeService) Open(session AccountSession) {
	service.authorization = "Bearer " + session.Token
	service.domains = MailboxUtils{}.Domains(session)
}

// @OnEvent({"replay":true})
func (service *UnreadBadgeService) Load(preferences UserPreferences) {
	service.names = MailboxUtils{}.Names(preferences.Values)
	service.show(service.normalized(preferences.Values))
}

func (service *UnreadBadgeService) DomainOptions() []DomainOption {
	return service.options()
}

func (service *UnreadBadgeService) SetScope(scope string) error {
	first := PreferenceUtils{}.Joined(service.domains[:min(1, len(service.domains))])
	known := slices.Contains([]string{service.everyAccount(), service.oneAccount()}, scope)
	FlowUtils{}.When(known, func() { service.save(scope, FlowUtils{}.Text(service.Domain != "", service.Domain, first)) })
	return nil
}

func (service *UnreadBadgeService) SetDomain(domain string) error {
	FlowUtils{}.When(slices.Contains(service.domains, domain), func() { service.save(service.oneAccount(), domain) })
	return nil
}

func (service *UnreadBadgeService) save(scope string, domain string) {
	next := UnreadBadgeUtils{}.Normalize(scope, domain, service.domains)
	refused := FlowUtils{}.Both(scope == service.oneAccount(), next.Scope != service.oneAccount())
	FlowUtils{}.Choose(refused, func() { service.Error = "Choose an account assigned to your profile." }, func() {
		FlowUtils{}.When(!service.Saving, func() { service.send(next) })
	})
}

func (service *UnreadBadgeService) send(next UnreadBadgePreference) {
	previous := UnreadBadgePreference{Scope: service.Scope, Domain: service.Domain}
	service.apply(next)
	service.Saving = true
	service.Error = ""
	stored, failure := service.usersClient.SavePreferences(service.authorization, map[string]any{service.scopeKey(): next.Scope, service.domainKey(): next.Domain})
	service.Saving = false
	FlowUtils{}.Choose(failure == nil, func() {
		service.apply(service.normalized(PreferenceUtils{}.Object(stored)))
		service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Unread badge preference saved."}, false)
	}, func() {
		service.apply(previous)
		service.Error = "Could not save unread badge preference."
	})
}

func (service *UnreadBadgeService) normalized(values map[string]any) UnreadBadgePreference {
	return UnreadBadgeUtils{}.Normalize(PreferenceUtils{}.Text(values, service.scopeKey()), PreferenceUtils{}.Text(values, service.domainKey()), service.domains)
}

func (service *UnreadBadgeService) show(preference UnreadBadgePreference) {
	service.Scope = preference.Scope
	service.Domain = preference.Domain
}

func (service *UnreadBadgeService) apply(preference UnreadBadgePreference) {
	service.show(preference)
	service.applicationEventPublisher.PublishEvent(preference, false)
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

func (service *UnreadBadgeService) scopeKey() string { return "unreadBadgeScope" }

func (service *UnreadBadgeService) domainKey() string { return "unreadBadgeDomain" }
