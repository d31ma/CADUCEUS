import (
	"maps"
	"slices"
	"strconv"
)

// MailPreferencesService is the "Mail preferences" section: workspace,
// reading, composing, sending, automatic recipients, snooze and contact
// learning. Edits collect in a draft and save as one profile.
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
func (service *MailPreferencesService) Open(session AccountSession) {
	service.authorization = "Bearer " + session.Token
	service.domains = MailboxUtils{}.Domains(session)
}

// @OnEvent({"replay":true})
func (service *MailPreferencesService) Load(preferences UserPreferences) {
	service.names = MailboxUtils{}.Names(preferences.Values)
	service.show(MailPreferenceUtils{}.Normalize(preferences.Values[service.key()]))
}

// @OnEvent({"replay":true})
func (service *MailPreferencesService) Learn(addresses SenderAddresses) {
	service.senders = addresses.Addresses
}

// Choose records one select or checkbox of the draft by its preference name.
func (service *MailPreferencesService) Choose(name string, value any) {
	values := map[string]any{}
	PreferenceUtils{}.Decode(service.draft, &values)
	values[name] = service.typed(value)
	PreferenceUtils{}.Decode(values, &service.draft)
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
	FlowUtils{}.When(!service.Saving, service.save)
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

func (service *MailPreferencesService) save() {
	next := MailPreferenceUtils{}.Normalize(service.draft)
	service.Saving = true
	service.Error = ""
	service.Status = ""
	stored, failure := service.usersClient.SavePreferences(service.authorization, map[string]any{service.key(): next})
	service.Saving = false
	service.showSaved(PreferenceUtils{}.Object(stored), failure)
}

func (service *MailPreferencesService) showSaved(stored map[string]any, failure error) {
	if failure != nil {
		service.Error = "Could not save mail preferences."
		return
	}
	service.show(MailPreferenceUtils{}.Normalize(stored[service.key()]))
	service.Status = "Mail preferences saved."
	service.applicationEventPublisher.PublishEvent(service.Current, false)
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: service.Status}, false)
}

// show replaces the shown profile and starts a fresh draft from it.
func (service *MailPreferencesService) show(preferences MailPreferences) {
	service.Current = preferences
	service.draft = MailPreferenceUtils{}.Normalize(preferences)
}

// typed reads the text of a numeric select as a number; other values stay.
func (service *MailPreferencesService) typed(value any) any {
	text, isText := value.(string)
	number, failure := strconv.ParseFloat(text, 64)
	if isText && failure == nil {
		return number
	}
	return value
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

func (service *MailPreferencesService) key() string { return "mailPreferences" }
