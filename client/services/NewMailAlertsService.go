import "slices"

// NewMailAlertsService chooses which hosted and connected accounts may send
// new-mail alerts to this device, how much a lock screen shows, and whether
// alerts carry Mark read and Archive actions.
// @Service
type NewMailAlertsService struct {
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	mailboxDomains            []string
	names                     map[string]string
	connected                 []ExternalAccount
	domains                   []string
	everyDomain               bool
	accounts                  []string
	everyAccount              bool
	Privacy                   string
	Actions                   bool
	Saving                    bool
	Error                     string
}

// @OnStart
func (service *NewMailAlertsService) Start() {
	service.apply(map[string]any{})
}

// @OnEvent({"replay":true})
func (service *NewMailAlertsService) Open(session AccountSession) {
	service.authorization = "Bearer " + session.Token
	service.mailboxDomains = MailboxUtils{}.Domains(session)
}

// @OnEvent({"replay":true})
func (service *NewMailAlertsService) Load(preferences UserPreferences) {
	service.names = MailboxUtils{}.Names(preferences.Values)
	service.apply(preferences.Values)
}

// @OnEvent({"replay":true})
func (service *NewMailAlertsService) Connect(list ConnectedAccountList) {
	service.connected = list.Accounts
}

func (service *NewMailAlertsService) DomainOptions() []DomainToggleRow {
	return service.domainOptions(service.enabledDomains())
}

func (service *NewMailAlertsService) AccountOptions() []ExternalAlertOption {
	return service.accountOptions(service.enabledAccounts())
}

func (service *NewMailAlertsService) ToggleDomain(domain string) error {
	service.save(map[string]any{service.domainsKey(): PreferenceUtils{}.Toggle(service.enabledDomains(), domain)})
	return nil
}

func (service *NewMailAlertsService) ToggleAccount(id string) error {
	service.save(map[string]any{service.accountsKey(): PreferenceUtils{}.Toggle(service.enabledAccounts(), id)})
	return nil
}

func (service *NewMailAlertsService) SetPrivacy(privacy string) error {
	FlowUtils{}.When(slices.Contains(service.privacyLevels(), privacy), func() { service.save(map[string]any{service.privacyKey(): privacy}) })
	return nil
}

func (service *NewMailAlertsService) SetActions(actions bool) error {
	service.save(map[string]any{service.actionsKey(): actions})
	return nil
}

func (service *NewMailAlertsService) save(patch map[string]any) {
	FlowUtils{}.When(!service.Saving, func() { service.send(patch) })
}

func (service *NewMailAlertsService) send(patch map[string]any) {
	restore := service.current()
	service.Saving, service.Error = true, ""
	service.apply(service.merged(patch))
	stored, failure := service.usersClient.SavePreferences(service.authorization, patch)
	service.Saving = false
	FlowUtils{}.Choose(failure == nil, func() {
		service.apply(PreferenceUtils{}.Object(stored))
		service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "New-mail notification preference saved."}, false)
	}, func() {
		service.Error = "Could not save new-mail notifications."
		service.apply(restore)
	})
}

func (service *NewMailAlertsService) apply(values map[string]any) {
	service.everyDomain = !PreferenceUtils{}.Present(values, service.domainsKey())
	service.domains = PreferenceUtils{}.Unique(PreferenceUtils{}.Strings(values, service.domainsKey()))
	service.everyAccount = !PreferenceUtils{}.Present(values, service.accountsKey())
	service.accounts = PreferenceUtils{}.Strings(values, service.accountsKey())
	service.Privacy = PreferenceUtils{}.Choice(PreferenceUtils{}.Text(values, service.privacyKey()), service.privacyLevels(), service.privacyLevels()[0])
	actions, chosen := values[service.actionsKey()].(bool)
	service.Actions = FlowUtils{}.Either(actions, !chosen)
}

// current writes the effective policy in the shape the server stores.
func (service *NewMailAlertsService) current() map[string]any {
	return map[string]any{
		service.domainsKey(): service.enabledDomains(), service.accountsKey(): service.enabledAccounts(),
		service.privacyKey(): service.Privacy, service.actionsKey(): service.Actions,
	}
}

func (service *NewMailAlertsService) merged(patch map[string]any) map[string]any {
	merged := service.current()
	for key, value := range patch {
		merged[key] = value
	}
	return merged
}

// enabledDomains are the hosted domains that alert: all of them until the
// account chose, and never one that the account no longer holds.
func (service *NewMailAlertsService) enabledDomains() []string {
	allowed := service.mailboxDomains
	requested := map[bool][]string{true: allowed, false: service.domains}[service.everyDomain]
	return slices.DeleteFunc(slices.Clone(requested), func(domain string) bool { return !slices.Contains(allowed, domain) })
}

func (service *NewMailAlertsService) enabledAccounts() []string {
	allowed := service.accountIds()
	requested := map[bool][]string{true: allowed, false: service.accounts}[service.everyAccount]
	return slices.DeleteFunc(slices.Clone(requested), func(id string) bool { return !slices.Contains(allowed, id) })
}

func (service *NewMailAlertsService) accountIds() []string {
	ids := []string{}
	for _, account := range service.connected {
		ids = append(ids, account.ID)
	}
	return ids
}

func (service *NewMailAlertsService) domainOptions(enabled []string) []DomainToggleRow {
	rows := []DomainToggleRow{}
	for _, domain := range service.mailboxDomains {
		rows = append(rows, DomainToggleRow{Domain: domain, Label: MailboxUtils{}.Label(service.names, domain), Enabled: slices.Contains(enabled, domain)})
	}
	return rows
}

func (service *NewMailAlertsService) accountOptions(enabled []string) []ExternalAlertOption {
	options := []ExternalAlertOption{}
	for _, account := range service.connected {
		options = append(options, ExternalAlertOption{ID: account.ID, Label: account.Label, Address: account.MailboxAddress, Enabled: slices.Contains(enabled, account.ID)})
	}
	return options
}

func (service *NewMailAlertsService) privacyLevels() []string { return []string{"full", "sender", "private"} }

func (service *NewMailAlertsService) domainsKey() string { return "newMailNotificationDomains" }

func (service *NewMailAlertsService) accountsKey() string { return "newMailNotificationExternalAccounts" }

func (service *NewMailAlertsService) privacyKey() string { return "newMailNotificationPrivacy" }

func (service *NewMailAlertsService) actionsKey() string { return "newMailNotificationActions" }
