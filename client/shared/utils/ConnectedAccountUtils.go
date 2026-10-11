import (
	"cmp"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// ConnectedAccountUtils holds the settings side of connected accounts: the
// provider catalog, the connection form, the card labels, and the OAuth
// redirect that brings the browser back to the settings page. The account
// fields that the sidebar shares stay in ExternalAccountUtils.
type ConnectedAccountUtils struct{}

// Providers keeps the catalog entries the settings form can offer.
func (utils ConnectedAccountUtils) Providers(providers []MailProvider) []MailProvider {
	return utils.guided(slices.DeleteFunc(slices.Clone(providers), utils.unsupported))
}

// Draft converts discovery output into the non-secret connection form values.
func (utils ConnectedAccountUtils) Draft(settings DiscoveredSettings, address string) map[string]string {
	connection := settings.Connection
	mailbox := PreferenceUtils{}.Lower(address)
	draft := map[string]string{}
	PreferenceUtils{}.Decode(ConnectionDraft{
		ChoiceID: cmp.Or(settings.ChoiceID, utils.names().Imap), Source: cmp.Or(settings.Source, utils.names().Manual),
		MailboxAddress: mailbox, Label: cmp.Or(strings.TrimSpace(settings.Label), mailbox),
		Username: cmp.Or(connection.Imap.Username, connection.Smtp.Username, mailbox),
		ImapHost: connection.Imap.Host, ImapPort: cmp.Or(utils.port(connection.Imap.Port), "993"),
		ImapSecurity: cmp.Or(connection.Imap.Security, "tls"),
		SmtpHost:     connection.Smtp.Host, SmtpPort: cmp.Or(utils.port(connection.Smtp.Port), "587"),
		SmtpSecurity: cmp.Or(connection.Smtp.Security, "starttls"),
		CardDavURL:   connection.Carddav.URL, CardDavUsername: connection.Carddav.Username,
		CalDavURL: connection.Caldav.URL, CalDavUsername: connection.Caldav.Username,
		ContainerFolder: connection.ContainerFolder,
	}, &draft)
	return draft
}

// PresetSettings is the discovery shape that selecting a catalog entry implies.
func (utils ConnectedAccountUtils) PresetSettings(provider MailProvider) DiscoveredSettings {
	preset := *cmp.Or(provider.Preset, &ProviderPreset{})
	source := map[bool]string{true: utils.names().Manual, false: "preset"}[provider.Preset == nil]
	return DiscoveredSettings{
		ChoiceID: provider.ID, Source: source,
		Connection: DiscoveredConnection{Imap: preset.Imap, Smtp: preset.Smtp, ContainerFolder: preset.ContainerFolder},
	}
}

// ProgressLabel describes an account's synchronization for its card.
func (utils ConnectedAccountUtils) ProgressLabel(account ExternalAccount) string {
	progress := *cmp.Or(account.Progress, &AccountProgress{})
	return cmp.Or(utils.syncing(progress.Phase, utils.count(progress)), utils.sentence(account.State))
}

// StateLabel writes a state such as "auth_required" as words.
func (ConnectedAccountUtils) StateLabel(state string) string {
	return strings.ReplaceAll(state, "_", " ")
}

func (utils ConnectedAccountUtils) CanPause(state string) bool {
	return slices.Contains(utils.activeStates(), state)
}

func (utils ConnectedAccountUtils) CanReconnect(state string) bool {
	return slices.Contains([]string{"auth_required", utils.activeStates()[3], "paused"}, state)
}

// OAuth is true for the providers that connect through OAuth.
func (utils ConnectedAccountUtils) OAuth(provider string) bool {
	return slices.Contains([]string{utils.names().Gmail, utils.names().Microsoft}, provider)
}

// Trusted is true when a provider sign-in address is the provider's own
// HTTPS host.
func (utils ConnectedAccountUtils) Trusted(provider string, address string) bool {
	parsed, failure := url.Parse(address)
	expected := map[bool]string{true: "accounts.google.com", false: "login.microsoftonline.com"}[provider == utils.names().Gmail]
	return utils.trusted(cmp.Or(parsed, &url.URL{}), failure, expected)
}

// Launch reads the provider redirect and the onboarding request from the
// address and from the keys the redirect left in storage.
func (utils ConnectedAccountUtils) Launch(location PageLocation, stored StoredValues) ProviderLaunch {
	query, _ := url.ParseQuery(strings.TrimPrefix(location.Search, "?"))
	names := utils.names()
	code, state := query.Get(names.Code), query.Get(names.State)
	return ProviderLaunch{
		Onboarding:  ChoiceUtils{}.Any(query.Get(names.Onboarding) == "accounts", stored.Local[names.Pending] == "1"),
		HasCallback: ChoiceUtils{}.Any(query.Has(names.Code), query.Has(names.State), query.Has(names.Error)),
		Failed:      ChoiceUtils{}.Any(query.Has(names.Error), code == "", state == ""),
		Code:        code, State: state,
		Provider: utils.provider(stored.Local[names.ProviderKey+":"+state], stored.Session[names.ProviderKey]),
		Cleaned:  utils.cleaned(location, query),
	}
}

// Diagnostics are the redacted synchronization details of one account.
func (ConnectedAccountUtils) Diagnostics(report AccountSyncReport, offsetMinutes float64) AccountDiagnostics {
	sync := report.Sync
	return AccountDiagnostics{
		CursorVersion: sync.CursorVersion, Attempts: sync.Attempts,
		NextRunAt: cmp.Or(DateLabelUtils{}.StoredDateTime(sync.NextRunAt, offsetMinutes), "Not scheduled"),
		LastCode:  sync.LastCode,
		UpdatedAt: DateLabelUtils{}.StoredDateTime(sync.ScheduleUpdatedAt, offsetMinutes),
	}
}

// Names are the provider and storage names of connected accounts.
func (ConnectedAccountUtils) Names() ConnectedAccountNames {
	return ConnectedAccountUtils{}.names()
}

func (utils ConnectedAccountUtils) unsupported(provider MailProvider) bool {
	known := slices.Contains([]string{utils.names().Gmail, utils.names().Microsoft, utils.names().Imap}, provider.Provider)
	authenticated := slices.Contains([]string{"oauth", "password"}, provider.Authentication)
	return slices.Contains([]bool{strings.TrimSpace(provider.ID) != "", strings.TrimSpace(provider.Name) != "", known, authenticated}, false)
}

func (ConnectedAccountUtils) guided(providers []MailProvider) []MailProvider {
	for index := range providers {
		providers[index].Guidance = cmp.Or(strings.TrimSpace(providers[index].Guidance), "none")
	}
	return providers
}

func (ConnectedAccountUtils) trusted(address *url.URL, failure error, expected string) bool {
	return !slices.Contains([]bool{failure == nil, address.Scheme == "https", address.Hostname() == expected}, false)
}

// port renders a stored port, or nothing when the endpoint had none.
func (utils ConnectedAccountUtils) port(port float64) string {
	return strings.TrimPrefix(utils.number(port), "0")
}

func (utils ConnectedAccountUtils) count(progress AccountProgress) string {
	completed := utils.number(progress.Completed)
	if progress.Total == nil {
		return completed
	}
	return completed + " of " + utils.number(*progress.Total)
}

func (ConnectedAccountUtils) number(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func (utils ConnectedAccountUtils) syncing(phase string, count string) string {
	if phase == "" {
		return ""
	}
	return "Syncing " + utils.StateLabel(phase) + " · " + count
}

func (utils ConnectedAccountUtils) sentence(state string) string {
	text := utils.StateLabel(state)
	return strings.ToUpper(text[:min(1, len(text))]) + text[min(1, len(text)):]
}

// provider is the provider remembered for this redirect's state, then for this tab.
func (utils ConnectedAccountUtils) provider(remembered string, session string) string {
	known := slices.DeleteFunc([]string{remembered, session}, func(name string) bool { return !utils.OAuth(name) })
	return strings.Join(append(known, "")[:1], "")
}

// cleaned is the address without the provider's secrets and the onboarding request.
func (utils ConnectedAccountUtils) cleaned(location PageLocation, query url.Values) string {
	names := utils.names()
	for _, name := range []string{names.Code, names.State, names.Error, "error_description", "error_uri", "session_state", names.Onboarding} {
		query.Del(name)
	}
	search := map[bool]string{true: "", false: "?" + query.Encode()}[len(query) == 0]
	return location.Path + search + location.Hash
}

func (ConnectedAccountUtils) activeStates() []string {
	return []string{"ready", "syncing", "idle", "degraded"}
}

func (ConnectedAccountUtils) names() ConnectedAccountNames {
	return ConnectedAccountNames{
		Gmail: "gmail", Microsoft: "microsoft", Imap: "imap", Manual: "manual",
		Pending: "caduceus_account_onboarding_pending", ProviderKey: "caduceus_oauth_provider",
		Completed: "caduceus_account_onboarding_complete",
		Code:      "code", State: "state", Error: "error", Onboarding: "onboarding",
	}
}
