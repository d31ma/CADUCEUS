import (
	"maps"
	"slices"
	"strconv"
)

// MailPreferencesService is the "Mail preferences" section: workspace,
// reading, composing, sending, automatic recipients, snooze and contact
// learning. Edits collect in a draft and save as one profile; the saved
// profile reaches PreferenceService, which applies the layout at once.
// @Service
type MailPreferencesService struct {
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	senders                   []string
	domains                   []string
	names                     map[string]string
	draft                     MailPreferences
	Current                   MailPreferences
	Saving                    bool
	Error                     string
	Status                    string
}

// @OnStart
func (service *MailPreferencesService) Start() {
	service.show(MailPreferenceUtils{}.Defaults())
}

// @OnEvent({"replay":true})
func (service *MailPreferencesService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
	service.domains = MailboxUtils{}.Hosted(changed.Domains)
}

// @OnEvent({"replay":true})
func (service *MailPreferencesService) Load(preferences UserPreferences) {
	service.names = preferences.MailboxNames
	service.show(MailPreferenceUtils{}.Normalize(preferences.MailPreferences))
}

// @OnEvent({"replay":true})
func (service *MailPreferencesService) Learn(addresses SenderAddresses) {
	service.senders = addresses.Addresses
}

// Choose records one select of the draft by its preference name. A
// numeric choice, such as a delay or the interface scale, is kept as a number.
func (service *MailPreferencesService) Choose(name string, value string) {
	number, failure := strconv.ParseFloat(value, 64)
	service.record(name, map[bool]any{true: number, false: value}[failure == nil])
}

// Toggle records one checkbox of the draft by its preference name.
func (service *MailPreferencesService) Toggle(name string, enabled bool) {
	service.record(name, enabled)
}

// SetRecipients records the automatic CC or BCC list of one sender.
func (service *MailPreferencesService) SetRecipients(sender string, kind string, value string) {
	recipients := map[string]map[string][]string{"cc": service.draft.AutoCcBySender, "bcc": service.draft.AutoBccBySender}[kind]
	recipients[sender] = MailPreferenceUtils{}.Recipients(value)
	maps.DeleteFunc(recipients, func(sender string, addresses []string) bool { return len(addresses) == 0 })
}

// ToggleContactLearning turns "found in mail" contact learning off or on for one domain.
func (service *MailPreferencesService) ToggleContactLearning(domain string) {
	service.draft.FindInMailDisabledDomains = PreferenceUtils{}.Toggle(service.draft.FindInMailDisabledDomains, domain)
}

func (service *MailPreferencesService) Save() error {
	FlowUtils{}.When(!service.Saving, service.save)()
	return nil
}

func (service *MailPreferencesService) SenderRows() []SenderRecipientRow {
	return service.senderRows(service.senders)
}

func (service *MailPreferencesService) DomainRows() []DomainToggleRow {
	return service.domainRows(service.domains)
}

func (service *MailPreferencesService) SpellcheckLanguages() []LanguageOption {
	return MailPreferenceUtils{}.SpellcheckLanguages()
}

func (service *MailPreferencesService) record(name string, value any) {
	values := map[string]any{}
	PreferenceUtils{}.Decode(service.draft, &values)
	values[name] = value
	PreferenceUtils{}.Decode(values, &service.draft)
}

func (service *MailPreferencesService) save() {
	next := MailPreferenceUtils{}.Normalize(&service.draft)
	service.Saving = true
	service.Error = ""
	service.Status = ""
	stored, failure := service.usersClient.SavePreferences(service.authorization, map[string]any{"mailPreferences": next})
	service.Saving = false
	FlowUtils{}.Either(failure == nil, func() { service.saved(stored) }, func() {
		service.Error = HTTPUtils{}.Message(failure, "Could not save mail preferences.", "Network error.")
	})()
}

func (service *MailPreferencesService) saved(stored UserPreferences) {
	service.show(MailPreferenceUtils{}.Normalize(stored.MailPreferences))
	service.Status = "Mail preferences saved."
	chosen := service.Current
	service.applicationEventPublisher.PublishEvent(PreferencesChosen{MailPreferences: &chosen}, false)
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: service.Status}, false)
}

// show replaces the shown profile and starts a fresh draft from it.
func (service *MailPreferencesService) show(preferences MailPreferences) {
	service.Current = preferences
	service.draft = MailPreferenceUtils{}.Normalize(&preferences)
}

func (service *MailPreferencesService) senderRows(senders []string) []SenderRecipientRow {
	rows := []SenderRecipientRow{}
	for _, sender := range senders {
		rows = append(rows, SenderRecipientRow{
			Address: sender,
			Cc:      PreferenceUtils{}.Joined(service.Current.AutoCcBySender[sender]),
			Bcc:     PreferenceUtils{}.Joined(service.Current.AutoBccBySender[sender]),
		})
	}
	return rows
}

func (service *MailPreferencesService) domainRows(domains []string) []DomainToggleRow {
	rows := []DomainToggleRow{}
	for _, domain := range domains {
		rows = append(rows, DomainToggleRow{Domain: domain, Label: MailboxUtils{}.Label(service.names, domain), Enabled: slices.Contains(service.Current.FindInMailDisabledDomains, domain)})
	}
	return rows
}
