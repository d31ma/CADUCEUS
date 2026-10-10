import (
	"cmp"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ExternalAccountUtils is the Go form of shared/scripts/external-accounts-ui.js:
// it keeps only the redacted account fields and provider catalog entries the
// settings view renders, and turns discovery output into a form draft.
type ExternalAccountUtils struct{}

// NormalizeAccounts keeps at most fifty well-formed accounts, ordered for display.
func (utils ExternalAccountUtils) NormalizeAccounts(value any) []ExternalAccount {
	items, _ := value.([]any)
	records := utils.records(items[:min(50, len(items))])
	return utils.sorted(utils.accounts(slices.DeleteFunc(records, utils.malformed)))
}

func (utils ExternalAccountUtils) NormalizeProviders(value any) []MailProvider {
	providers := []MailProvider{}
	PreferenceUtils{}.Decode(value, &providers)
	return utils.providers(slices.DeleteFunc(providers, utils.unsupported))
}

// Draft converts discovery output into the non-secret connection form values.
func (utils ExternalAccountUtils) Draft(settings DiscoveredSettings, address string) ConnectionDraft {
	connection := settings.Connection
	mailbox := PreferenceUtils{}.Lower(address)
	return ConnectionDraft{
		ChoiceID: cmp.Or(settings.ChoiceID, utils.providerKinds()[2]), Source: cmp.Or(settings.Source, utils.manual()),
		MailboxAddress: mailbox, Label: cmp.Or(strings.TrimSpace(settings.Label), mailbox),
		Username: cmp.Or(connection.Imap.Username, connection.Smtp.Username, mailbox),
		ImapHost: connection.Imap.Host, ImapPort: cmp.Or(utils.port(connection.Imap.Port), "993"),
		ImapSecurity: cmp.Or(connection.Imap.Security, "tls"),
		SmtpHost: connection.Smtp.Host, SmtpPort: cmp.Or(utils.port(connection.Smtp.Port), "587"),
		SmtpSecurity: cmp.Or(connection.Smtp.Security, "starttls"),
		CardDavURL: connection.Carddav.URL, CardDavUsername: connection.Carddav.Username,
		CalDavURL: connection.Caldav.URL, CalDavUsername: connection.Caldav.Username,
		ContainerFolder: connection.ContainerFolder,
	}
}

// PresetSettings is the discovery shape that selecting a catalog entry implies.
func (utils ExternalAccountUtils) PresetSettings(provider MailProvider) DiscoveredSettings {
	preset := utils.preset(provider.Preset)
	return DiscoveredSettings{
		ChoiceID: provider.ID, Source: utils.presetSource(provider.Preset),
		Connection: DiscoveredConnection{Imap: preset.Imap, Smtp: preset.Smtp, ContainerFolder: preset.ContainerFolder},
	}
}

// ProgressLabel describes an account's synchronization for its card.
func (utils ExternalAccountUtils) ProgressLabel(account ExternalAccount) string {
	progress := utils.progress(account.Progress)
	return cmp.Or(utils.syncing(progress.Phase, utils.count(progress)), utils.stateLabel(account.State))
}

func (utils ExternalAccountUtils) StateLabel(state string) string {
	return utils.words(state)
}

func (utils ExternalAccountUtils) CanPause(state string) bool {
	return slices.Contains(utils.activeStates(), state)
}

func (utils ExternalAccountUtils) CanReconnect(state string) bool {
	return slices.Contains([]string{utils.authRequired(), utils.activeStates()[3], utils.paused()}, state)
}

func (ExternalAccountUtils) records(items []any) []ExternalAccountRecord {
	records := []ExternalAccountRecord{}
	for _, item := range items {
		record := ExternalAccountRecord{}
		PreferenceUtils{}.Decode(item, &record)
		records = append(records, record)
	}
	return records
}

func (utils ExternalAccountUtils) malformed(record ExternalAccountRecord) bool {
	states := append(utils.activeStates(), utils.authRequired(), utils.paused(), "deleting")
	whole := record.Revision == float64(int64(record.Revision))
	if !utils.idPattern().MatchString(strings.TrimSpace(record.ID)) || strings.TrimSpace(record.MailboxAddress) == "" || !slices.Contains(utils.providerKinds(), record.Provider) || !slices.Contains(states, record.State) || record.Revision < 1 || !whole {
		return true
	}
	return false
}

func (utils ExternalAccountUtils) accounts(records []ExternalAccountRecord) []ExternalAccount {
	accounts := []ExternalAccount{}
	color := utils.colorPattern()
	for _, record := range records {
		mailbox := strings.TrimSpace(record.MailboxAddress)
		accounts = append(accounts, ExternalAccount{
			ID: strings.TrimSpace(record.ID), MailboxAddress: mailbox, Provider: record.Provider, State: record.State,
			Revision: record.Revision, AuthIdentity: strings.TrimSpace(record.AuthIdentity),
			SharedMailboxAddress: strings.TrimSpace(record.SharedMailboxAddress),
			Label: cmp.Or(strings.TrimSpace(record.Label), mailbox),
			Color: cmp.Or(color.FindString(strings.TrimSpace(record.Color)), "#5f6368"),
			Order: record.Order, CanReadMail: record.Capabilities.MailRead, CanSend: record.Capabilities.Send,
			ImapUsername: record.Connection.Imap.Username, Progress: record.Progress, Failure: record.Failure,
			UpdatedAt: strings.TrimSpace(record.UpdatedAt),
		})
	}
	return accounts
}

func (utils ExternalAccountUtils) unsupported(provider MailProvider) bool {
	if strings.TrimSpace(provider.ID) == "" || strings.TrimSpace(provider.Name) == "" || !slices.Contains(utils.providerKinds(), provider.Provider) || !slices.Contains([]string{"oauth", "password"}, provider.Authentication) {
		return true
	}
	return false
}

func (ExternalAccountUtils) providers(providers []MailProvider) []MailProvider {
	for index := range providers {
		providers[index].Guidance = cmp.Or(strings.TrimSpace(providers[index].Guidance), "none")
	}
	return providers
}

func (ExternalAccountUtils) preset(preset *ProviderPreset) ProviderPreset {
	if preset == nil {
		return ProviderPreset{}
	}
	return *preset
}

func (utils ExternalAccountUtils) presetSource(preset *ProviderPreset) string {
	if preset == nil {
		return utils.manual()
	}
	return "preset"
}

// port renders a stored port, or nothing when the endpoint had none.
func (utils ExternalAccountUtils) port(port float64) string {
	return strings.TrimPrefix(utils.number(port), "0")
}

func (ExternalAccountUtils) progress(progress *AccountProgress) AccountProgress {
	if progress == nil {
		return AccountProgress{}
	}
	return *progress
}

func (utils ExternalAccountUtils) count(progress AccountProgress) string {
	completed := utils.number(progress.Completed)
	if progress.Total == nil {
		return completed
	}
	return completed + " of " + utils.number(*progress.Total)
}

func (utils ExternalAccountUtils) syncing(phase string, count string) string {
	if phase == "" {
		return ""
	}
	return "Syncing " + utils.words(phase) + " · " + count
}

func (utils ExternalAccountUtils) stateLabel(state string) string {
	text := utils.words(state)
	return strings.ToUpper(text[:min(1, len(text))]) + text[min(1, len(text)):]
}

func (ExternalAccountUtils) idPattern() *regexp.Regexp {
	return regexp.MustCompile(`^ext_[A-Za-z0-9_-]{20,80}$`)
}

func (ExternalAccountUtils) colorPattern() *regexp.Regexp {
	return regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
}

// Including adds one account that the server just created, in display order.
func (utils ExternalAccountUtils) Including(accounts []ExternalAccount, value any) []ExternalAccount {
	return utils.sorted(append(slices.Clone(accounts), utils.NormalizeAccounts([]any{value})...))
}

// Replacing swaps in the server's newer copy of one account.
func (utils ExternalAccountUtils) Replacing(accounts []ExternalAccount, value any) []ExternalAccount {
	replacement := utils.NormalizeAccounts([]any{value})
	kept := slices.DeleteFunc(slices.Clone(accounts), func(account ExternalAccount) bool {
		return slices.ContainsFunc(replacement, func(candidate ExternalAccount) bool { return candidate.ID == account.ID })
	})
	return utils.sorted(append(kept, replacement...))
}

func (ExternalAccountUtils) sorted(accounts []ExternalAccount) []ExternalAccount {
	slices.SortStableFunc(accounts, func(left ExternalAccount, right ExternalAccount) int {
		return cmp.Or(cmp.Compare(left.Order, right.Order), strings.Compare(left.Label, right.Label))
	})
	return accounts
}

func (ExternalAccountUtils) providerKinds() []string { return []string{"gmail", "microsoft", "imap"} }

func (ExternalAccountUtils) activeStates() []string { return []string{"ready", "syncing", "idle", "degraded"} }

func (ExternalAccountUtils) authRequired() string { return "auth_required" }

func (ExternalAccountUtils) paused() string { return "paused" }

func (ExternalAccountUtils) manual() string { return "manual" }

func (ExternalAccountUtils) number(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func (ExternalAccountUtils) words(text string) string {
	return strings.ReplaceAll(text, "_", " ")
}
