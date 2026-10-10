import (
	"cmp"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// ConnectedAccountsService is the "Connected accounts" section: outside
// mailboxes, the guided first-run setup, provider discovery, password and
// OAuth connection, and the pause, resume, retry, reconnect and remove
// actions. Passwords live only in memory until the request that uses them.
// @Service
type ConnectedAccountsService struct {
	// @Needs
	externalAccountsClient    *ExternalAccountsClient
	applicationEventPublisher ApplicationEventPublisher
	timeZoneOffset            float64
	authorization             string
	launch                    ProviderLaunch
	callbackHandled           bool
	diagnostics               map[string]AccountDiagnostics
	Accounts                  []ExternalAccount
	Providers                 []MailProvider
	AccountsError             string
	OAuthNotice               string
	// @SessionState
	SetupOpen bool
	// @SessionState
	Onboarding bool
	// @SessionState
	SetupStep string
	// @SessionState
	Address string
	// @SessionState
	Draft             map[string]string
	SetupStatus       string
	SetupError        string
	SetupBusy         bool
	mutationID        string
	reconnectID       string
	removeID          string
	diagnosticsID     string
	Password          string
	ReconnectUsername string
	ReconnectPassword string
}

// @OnStart
func (service *ConnectedAccountsService) Start() {
	service.diagnostics = map[string]AccountDiagnostics{}
	service.SetupStep = cmp.Or(service.SetupStep, service.names().Address)
	FlowUtils{}.When(service.Draft == nil, func() { service.Draft = service.emptyDraft("") })
}

// @OnEvent({"replay":true})
func (service *ConnectedAccountsService) Open(session AccountSession) error {
	service.authorization = "Bearer " + session.Token
	service.resume()
	service.load()
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: service.names().Section}, false)
	return nil
}

// @OnEvent({"replay":true})
func (service *ConnectedAccountsService) Learn(platform DevicePlatform) {
	service.timeZoneOffset = platform.TimeZoneOffset
}

// @OnEvent({"replay":true})
func (service *ConnectedAccountsService) Arrive(launch ProviderLaunch) error {
	service.launch = launch
	FlowUtils{}.When(launch.Onboarding, service.StartOnboarding)
	service.resume()
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: service.names().Section}, false)
	return nil
}

func (service *ConnectedAccountsService) OpenSetup() {
	service.SetupOpen = true
	service.SetupStep = service.names().Address
	service.SetupError = ""
	service.SetupStatus = ""
	service.Address = ""
	service.Draft = service.emptyDraft("")
}

func (service *ConnectedAccountsService) StartOnboarding() {
	service.Onboarding = true
	service.applicationEventPublisher.PublishEvent(AccountOnboardingChanged{Pending: true}, false)
	service.OpenSetup()
}

func (service *ConnectedAccountsService) CloseSetupOrOnboarding() {
	FlowUtils{}.Choose(service.Onboarding, service.dismissOnboarding, service.closeSetup)
}

// SelectProvider moves the setup to the OAuth or credentials step of one catalog entry.
func (service *ConnectedAccountsService) SelectProvider(id string) {
	provider := service.provider(id)
	FlowUtils{}.Choose(provider.Authentication == service.names().OAuth, func() { service.selectOAuth(provider) }, func() { service.selectCredentials(provider) })
}

func (service *ConnectedAccountsService) SetField(name string, value string) {
	draft := map[string]string{}
	PreferenceUtils{}.Decode(service.Draft, &draft)
	draft[name] = value
	service.Draft = draft
}

// Discover asks the server for the mail settings of the entered address.
func (service *ConnectedAccountsService) Discover() error {
	address := PreferenceUtils{}.Lower(service.Address)
	valid := service.complete(address)
	FlowUtils{}.Choose(valid, func() { service.discover(address) }, func() { service.SetupError = "Enter a complete email address." })
	return nil
}

// BeginOAuth starts provider authorization for the mailbox in the OAuth step.
func (service *ConnectedAccountsService) BeginOAuth() error {
	identity := PreferenceUtils{}.Lower(service.Draft["authIdentity"])
	shared := PreferenceUtils{}.Lower(service.Draft[service.names().Shared])
	complete := FlowUtils{}.Both(service.complete(identity), FlowUtils{}.Either(shared == "", service.complete(shared)))
	FlowUtils{}.Choose(complete, func() { service.beginNew(identity, shared) }, func() {
		service.SetupError = "Enter a complete provider sign-in address and, when used, a complete shared mailbox address."
	})
	return nil
}

// ValidateAndCreate checks the servers without sending mail, then saves the account.
func (service *ConnectedAccountsService) ValidateAndCreate() error {
	request, problem := service.credentialsRequest()
	service.Password = ""
	FlowUtils{}.Choose(problem == nil, func() { service.create(request) }, func() { service.SetupError = problem.Error() })
	return nil
}

func (service *ConnectedAccountsService) Mutate(id string, action string) error {
	account := service.account(id)
	answer, failure := service.externalAccountsClient.Change(service.authorization, id, service.withRevision(account, map[string]any{"action": action}))
	service.showReplaced(answer, failure, "Could not "+action+" this account.")
	return nil
}

func (service *ConnectedAccountsService) Retry(id string) error {
	account := service.account(id)
	answer, failure := service.externalAccountsClient.Retry(service.authorization, id, service.revisionBody(account))
	scheduled := PreferenceUtils{}.Flag(PreferenceUtils{}.Object(answer), "scheduled")
	FlowUtils{}.Choose(FlowUtils{}.Both(failure == nil, scheduled), func() {
		service.toast("Account retry scheduled.")
		service.loadDiagnostics(id)
	}, func() { service.AccountsError = "Could not schedule an account retry." })
	return nil
}

// ToggleDiagnostics opens or closes the redacted synchronization details.
func (service *ConnectedAccountsService) ToggleDiagnostics(id string) error {
	FlowUtils{}.Choose(service.diagnosticsID == id, func() { service.diagnosticsID = "" }, func() { service.loadDiagnostics(id) })
	return nil
}

func (service *ConnectedAccountsService) OpenReconnect(id string) {
	account := service.account(id)
	service.reconnectID = id
	service.removeID = ""
	service.ReconnectUsername = cmp.Or(account.ImapUsername, account.MailboxAddress)
	service.ReconnectPassword = ""
}

func (service *ConnectedAccountsService) CancelReconnect() {
	service.reconnectID = ""
	service.ReconnectPassword = ""
}

// Reconnect authorizes an OAuth account again or replaces a password.
func (service *ConnectedAccountsService) Reconnect(id string) error {
	account := service.account(id)
	oauth := slices.Contains([]string{service.names().Microsoft, service.names().Gmail}, account.Provider)
	FlowUtils{}.Choose(oauth, func() { service.beginReconnect(account) }, func() { service.reconnectPassword(account) })
	return nil
}

func (service *ConnectedAccountsService) AskRemove(id string) {
	service.removeID = id
	service.reconnectID = ""
}

func (service *ConnectedAccountsService) CancelRemove() {
	service.removeID = ""
}

func (service *ConnectedAccountsService) Remove(id string) error {
	account := service.account(id)
	_, failure := service.externalAccountsClient.Remove(service.authorization, id, service.revisionBody(account))
	FlowUtils{}.Choose(failure == nil, func() { service.markRemoving(id) }, func() { service.failMutation("Could not remove this account.") })
	return nil
}

// Rows are the account cards, with the state of their inline panels.
func (service *ConnectedAccountsService) Rows() []ExternalAccountRow {
	return service.rows()
}

func (service *ConnectedAccountsService) rows() []ExternalAccountRow {
	rows := []ExternalAccountRow{}
	for _, account := range service.Accounts {
		rows = append(rows, service.row(account))
	}
	return rows
}

func (service *ConnectedAccountsService) SelectedProvider() MailProvider {
	return service.provider(service.Draft[service.names().Choice])
}

func (service *ConnectedAccountsService) SelectedGuidance() string {
	guidance := map[string]string{
		"app_password": "This provider requires an app-specific password.",
		"enable_imap":  "Enable IMAP access in the provider before connecting.",
	}
	return guidance[service.SelectedProvider().Guidance]
}

func (service *ConnectedAccountsService) load() {
	accounts, accountsFailure := service.externalAccountsClient.Accounts(service.authorization)
	providers, providersFailure := service.externalAccountsClient.Providers(service.authorization)
	service.AccountsError = ""
	service.showLoaded(accounts, providers, errors.Join(accountsFailure, providersFailure))
}

func (service *ConnectedAccountsService) showLoaded(accounts BehaviorValue, providers BehaviorValue, failure error) {
	if failure != nil {
		service.AccountsError = "Could not load connected accounts."
		return
	}
	service.share(ExternalAccountUtils{}.NormalizeAccounts(PreferenceUtils{}.Object(accounts)["accounts"]))
	service.Providers = ExternalAccountUtils{}.NormalizeProviders(PreferenceUtils{}.Object(providers)["providers"])
}

// resume finishes a provider redirect once both the session and the
// callback parameters have arrived, in whichever order they came.
func (service *ConnectedAccountsService) resume() {
	ready := FlowUtils{}.Both(service.authorization != "", service.launch.HasCallback)
	FlowUtils{}.When(FlowUtils{}.Both(ready, !service.callbackHandled), service.finishAuthorization)
}

func (service *ConnectedAccountsService) finishAuthorization() {
	service.callbackHandled = true
	FlowUtils{}.Choose(service.launch.Failed, func() {
		service.OAuthNotice = "Provider authorization was not completed. No account credentials were changed."
	}, service.completeAuthorization)
}

func (service *ConnectedAccountsService) completeAuthorization() {
	body := map[string]any{"code": service.launch.Code, "state": service.launch.State}
	answer, failure := service.completeWith(cmp.Or(service.launch.Provider, service.names().Microsoft), body)
	answer, failure = service.completeWithGoogle(answer, failure, body)
	service.showAuthorized(answer, failure)
}

func (service *ConnectedAccountsService) completeWith(provider string, body BehaviorValue) (BehaviorValue, error) {
	if provider == service.names().Gmail {
		return service.externalAccountsClient.CompleteGoogle(service.authorization, body)
	}
	return service.externalAccountsClient.CompleteMicrosoft(service.authorization, body)
}

// completeWithGoogle tries Google after Microsoft when no provider was remembered.
func (service *ConnectedAccountsService) completeWithGoogle(answer BehaviorValue, failure error, body BehaviorValue) (BehaviorValue, error) {
	if failure != nil && service.launch.Provider == "" {
		return service.externalAccountsClient.CompleteGoogle(service.authorization, body)
	}
	return answer, failure
}

func (service *ConnectedAccountsService) showAuthorized(answer BehaviorValue, failure error) {
	if failure != nil {
		service.OAuthNotice = "Provider authorization could not be completed. Try reconnecting the account."
		return
	}
	body := PreferenceUtils{}.Object(answer)
	kind := FlowUtils{}.Text(PreferenceUtils{}.Text(body, service.names().Provider) == service.names().Gmail, "Google account", "Microsoft account")
	service.OAuthNotice = FlowUtils{}.Text(PreferenceUtils{}.Text(body, service.names().Shared) != "", "Shared mailbox", kind) + " connected successfully."
	service.completeOnboarding()
	service.toast(service.OAuthNotice)
	service.applicationEventPublisher.PublishEvent(ExternalAccountsChanged{Count: len(service.Accounts)}, false)
	service.load()
}

func (service *ConnectedAccountsService) dismissOnboarding() {
	service.Onboarding = false
	service.applicationEventPublisher.PublishEvent(AccountOnboardingChanged{Completed: true}, false)
	service.closeSetup()
}

func (service *ConnectedAccountsService) completeOnboarding() {
	FlowUtils{}.When(service.Onboarding, func() {
		service.Onboarding = false
		service.SetupOpen = false
		service.applicationEventPublisher.PublishEvent(AccountOnboardingChanged{Completed: true}, false)
	})
}

func (service *ConnectedAccountsService) closeSetup() {
	service.SetupOpen = false
	service.SetupBusy = false
	service.SetupError = ""
	service.SetupStatus = ""
	service.Password = ""
	service.Draft = service.emptyDraft("")
}

func (service *ConnectedAccountsService) selectOAuth(provider MailProvider) {
	FlowUtils{}.Choose(provider.Configured, func() { service.enterOAuth(provider) }, func() {
		service.SetupError = provider.Name + " OAuth must be configured by this server’s administrator before this account can be connected."
	})
}

func (service *ConnectedAccountsService) enterOAuth(provider MailProvider) {
	address := PreferenceUtils{}.Lower(service.Address)
	draft := service.emptyDraft(address)
	draft[service.names().Choice] = provider.ID
	draft["oauthProvider"] = provider.Provider
	draft["authIdentity"] = address
	draft[service.names().Shared] = ""
	draft[service.names().Label] = cmp.Or(service.Address, provider.Name)
	draft[service.names().Color] = service.defaultColor(provider.Provider)
	service.SetupError = ""
	service.SetupStatus = ""
	service.SetupStep = service.names().OAuth
	service.Draft = draft
}

func (service *ConnectedAccountsService) selectCredentials(provider MailProvider) {
	service.SetupError = ""
	service.SetupStep = "credentials"
	service.Draft = service.draft(ExternalAccountUtils{}.PresetSettings(provider), service.Address)
}

func (service *ConnectedAccountsService) discover(address string) {
	service.Address = address
	service.SetupBusy = true
	service.SetupError = ""
	service.SetupStatus = "Discovering secure mail settings…"
	settings, failure := service.externalAccountsClient.Discover(service.authorization, map[string]any{service.names().Address: address})
	service.SetupBusy = false
	service.showDiscovered(settings, failure, address)
}

func (service *ConnectedAccountsService) showDiscovered(settings DiscoveredSettings, failure error, address string) {
	if failure != nil {
		service.SetupError = "Mail settings could not be discovered."
		service.SetupStatus = ""
		return
	}
	provider := service.provider(settings.ChoiceID)
	FlowUtils{}.Choose(provider.Authentication == service.names().OAuth, func() { service.discoveredOAuth(provider) }, func() { service.discoveredServers(settings, address) })
}

func (service *ConnectedAccountsService) discoveredOAuth(provider MailProvider) {
	service.selectOAuth(provider)
	entered := service.SetupStep == service.names().OAuth
	service.SetupError = FlowUtils{}.Text(entered, "", cmp.Or(service.SetupError, provider.Name+" OAuth is not available."))
	service.SetupStatus = FlowUtils{}.Text(entered, provider.Name+" settings found. Confirm the sign-in address to continue.", "")
}

func (service *ConnectedAccountsService) discoveredServers(settings DiscoveredSettings, address string) {
	service.Draft = service.draft(settings, address)
	service.SetupStep = "credentials"
	service.SetupStatus = FlowUtils{}.Text(settings.Source == "fallback",
		"Suggested settings found. Review every server field before connecting.",
		"Mail settings discovered. Add your credentials to continue.")
}

func (service *ConnectedAccountsService) beginNew(identity string, shared string) {
	provider := cmp.Or(service.SelectedProvider().Provider, service.Draft["oauthProvider"])
	mailbox := cmp.Or(shared, identity)
	request := map[string]any{
		service.names().Provider: provider, "authIdentity": identity, "mailboxAddress": mailbox, service.names().Shared: shared,
		"label": cmp.Or(strings.TrimSpace(service.Draft[service.names().Label]), mailbox),
		"color": cmp.Or(strings.TrimSpace(service.Draft[service.names().Color]), service.defaultColor(provider)),
		"order": len(service.Accounts),
	}
	service.SetupBusy = true
	service.SetupError = ""
	service.AccountsError = ""
	service.SetupStatus = service.preparing(provider)
	redirect, problem := service.authorize(provider, request)
	service.showRedirect(redirect, problem, func(message string) { service.SetupError = message })
}

func (service *ConnectedAccountsService) beginReconnect(account ExternalAccount) {
	service.mutationID = account.ID
	service.AccountsError = ""
	service.SetupStatus = service.preparing(account.Provider)
	request := service.withRevision(account, map[string]any{service.names().Provider: account.Provider, "accountId": account.ID})
	redirect, problem := service.authorize(account.Provider, request)
	service.showRedirect(redirect, problem, func(message string) { service.AccountsError = message })
}

// authorize asks the server for the provider's sign-in address and checks
// that it really is the provider's own HTTPS host.
func (service *ConnectedAccountsService) authorize(provider string, request map[string]any) (ProviderRedirectRequested, error) {
	answer, failure := service.beginWith(provider, request)
	body := PreferenceUtils{}.Object(answer)
	address := service.parsed(PreferenceUtils{}.Text(body, "authorizationUrl"))
	expected := FlowUtils{}.Text(provider == service.names().Gmail, "accounts.google.com", "login.microsoftonline.com")
	trusted := FlowUtils{}.Both(address.Scheme == "https", address.Hostname() == expected)
	redirect := ProviderRedirectRequested{URL: address.String(), Provider: provider, State: PreferenceUtils{}.Text(body, "state"), Onboarding: service.Onboarding}
	return redirect, cmp.Or(service.startFailure(failure), service.addressFailure(trusted))
}

func (service *ConnectedAccountsService) parsed(text string) *url.URL {
	address, failure := url.Parse(text)
	if failure != nil {
		return &url.URL{}
	}
	return address
}

func (service *ConnectedAccountsService) beginWith(provider string, request map[string]any) (BehaviorValue, error) {
	if provider == service.names().Gmail {
		return service.externalAccountsClient.BeginGoogle(service.authorization, request)
	}
	return service.externalAccountsClient.BeginMicrosoft(service.authorization, request)
}

func (service *ConnectedAccountsService) startFailure(failure error) error {
	if failure != nil {
		return errors.New("Provider authorization could not be started.")
	}
	return nil
}

func (service *ConnectedAccountsService) addressFailure(trusted bool) error {
	if !trusted {
		return errors.New("The server returned an invalid provider authorization address.")
	}
	return nil
}

func (service *ConnectedAccountsService) showRedirect(redirect ProviderRedirectRequested, problem error, report func(string)) {
	if problem != nil {
		report(problem.Error())
		service.SetupStatus = ""
		service.SetupBusy = false
		service.mutationID = ""
		return
	}
	service.applicationEventPublisher.PublishEvent(redirect, false)
}

// credentialsRequest builds one request from the form; the password is
// copied only into this request.
func (service *ConnectedAccountsService) credentialsRequest() (map[string]any, error) {
	draft := service.Draft
	mailbox := cmp.Or(draft["mailboxAddress"], PreferenceUtils{}.Lower(service.Address))
	username := strings.TrimSpace(draft[service.names().Username])
	imapPort, imapFailure := strconv.Atoi(strings.TrimSpace(draft["imapPort"]))
	smtpPort, smtpFailure := strconv.Atoi(strings.TrimSpace(draft["smtpPort"]))
	imapHost, smtpHost := strings.TrimSpace(draft["imapHost"]), strings.TrimSpace(draft["smtpHost"])
	request := map[string]any{
		service.names().Provider: "imap", "mailboxAddress": mailbox,
		"label": cmp.Or(strings.TrimSpace(draft["label"]), mailbox), "color": cmp.Or(strings.TrimSpace(draft["color"]), "#5f6368"),
		"order": len(service.Accounts), "containerFolder": strings.TrimSpace(draft["containerFolder"]),
		service.names().Credentials: service.credentials(username, service.Password),
		"imap":        service.endpoint(imapHost, imapPort, draft["imapSecurity"], username),
		"smtp":        service.endpoint(smtpHost, smtpPort, draft["smtpSecurity"], username),
	}
	service.addDirectory(request, service.names().CardDirectory, draft["cardDAVURL"], cmp.Or(strings.TrimSpace(draft["cardDAVUsername"]), username))
	service.addDirectory(request, service.names().CalendarDirectory, draft["calDAVURL"], cmp.Or(strings.TrimSpace(draft["calDAVUsername"]), username))
	missing := slices.Contains([]string{mailbox, username, service.Password}, "")
	servers := FlowUtils{}.Both(imapHost != "", smtpHost != "")
	return request, service.requestProblem(missing, FlowUtils{}.Both(servers, errors.Join(imapFailure, smtpFailure) == nil))
}

func (service *ConnectedAccountsService) endpoint(host string, port int, security string, username string) map[string]any {
	return map[string]any{"host": strings.TrimSpace(host), "port": port, "security": strings.TrimSpace(security), service.names().Username: username, service.names().Insecure: false}
}

func (service *ConnectedAccountsService) addDirectory(request map[string]any, key string, address string, username string) {
	if strings.TrimSpace(address) != "" {
		request[key] = map[string]any{"url": strings.TrimSpace(address), service.names().Username: username, service.names().Insecure: false}
	}
}

func (service *ConnectedAccountsService) requestProblem(missing bool, servers bool) error {
	switch {
	case missing:
		return errors.New("Email, username, and password are required.")
	case !servers:
		return errors.New("Complete both IMAP and SMTP server settings.")
	}
	return nil
}

func (service *ConnectedAccountsService) create(request map[string]any) {
	service.SetupBusy = true
	service.SetupError = ""
	service.SetupStatus = service.validatingStatus(request)
	_, validationFailure := service.externalAccountsClient.Validate(service.authorization, request)
	created, creationFailure := service.saveValidated(request, validationFailure)
	request[service.names().Credentials] = map[string]any{}
	service.SetupBusy = false
	service.showCreated(created, validationFailure, creationFailure)
}

func (service *ConnectedAccountsService) validatingStatus(request map[string]any) string {
	services := slices.DeleteFunc([]string{
		FlowUtils{}.Text(request[service.names().CardDirectory] != nil, "CardDAV", ""),
		FlowUtils{}.Text(request[service.names().CalendarDirectory] != nil, "CalDAV", ""),
	}, func(name string) bool { return name == "" })
	return FlowUtils{}.Text(len(services) > 0, "Validating IMAP, SMTP, and "+strings.Join(services, " and ")+" without sending mail…", "Validating IMAP and SMTP without sending mail…")
}

func (service *ConnectedAccountsService) saveValidated(request map[string]any, validationFailure error) (BehaviorValue, error) {
	if validationFailure != nil {
		return nil, validationFailure
	}
	service.SetupStatus = "Validation passed. Saving the encrypted account credential…"
	return service.externalAccountsClient.Create(service.authorization, request)
}

func (service *ConnectedAccountsService) showCreated(created BehaviorValue, validationFailure error, creationFailure error) {
	if creationFailure != nil {
		service.SetupError = FlowUtils{}.Text(validationFailure != nil, "The mail servers rejected these settings.", "The account could not be added.")
		service.SetupStatus = ""
		return
	}
	service.share(ExternalAccountUtils{}.Including(service.Accounts, created))
	service.completeOnboarding()
	service.closeSetup()
	service.toast("External account connected.")
}

func (service *ConnectedAccountsService) reconnectPassword(account ExternalAccount) {
	password := service.ReconnectPassword
	service.ReconnectPassword = ""
	FlowUtils{}.When(password != "", func() { service.sendReconnect(account, password) })
}

func (service *ConnectedAccountsService) sendReconnect(account ExternalAccount, password string) {
	service.mutationID = account.ID
	service.AccountsError = ""
	credentials := service.credentials(cmp.Or(strings.TrimSpace(service.ReconnectUsername), account.MailboxAddress), password)
	answer, failure := service.externalAccountsClient.Reconnect(service.authorization, account.ID, service.withRevision(account, map[string]any{service.names().Credentials: credentials}))
	service.showReplaced(answer, failure, "Could not reconnect this account.")
	FlowUtils{}.When(failure == nil, func() {
		service.reconnectID = ""
		service.toast("Account reconnected.")
	})
}

func (service *ConnectedAccountsService) showReplaced(answer BehaviorValue, failure error, message string) {
	service.mutationID = ""
	if failure != nil {
		service.failMutation(message)
		return
	}
	service.AccountsError = ""
	service.share(ExternalAccountUtils{}.Replacing(service.Accounts, answer))
}

func (service *ConnectedAccountsService) failMutation(message string) {
	service.mutationID = ""
	service.load()
	service.AccountsError = message
}

func (service *ConnectedAccountsService) markRemoving(id string) {
	index := slices.IndexFunc(service.Accounts, func(account ExternalAccount) bool { return account.ID == id })
	FlowUtils{}.When(index >= 0, func() { service.Accounts[index] = service.removing(service.Accounts[index]) })
	service.share(service.Accounts)
	service.removeID = ""
	service.toast("Account removal started.")
}

func (service *ConnectedAccountsService) removing(account ExternalAccount) ExternalAccount {
	account.State = "deleting"
	account.Revision += 1
	account.Progress = nil
	account.Failure = nil
	return account
}

func (service *ConnectedAccountsService) loadDiagnostics(id string) {
	service.diagnosticsID = id
	answer, failure := service.externalAccountsClient.Diagnostics(service.authorization, id)
	service.showDiagnostics(id, answer, failure)
}

func (service *ConnectedAccountsService) showDiagnostics(id string, answer BehaviorValue, failure error) {
	if failure != nil {
		service.AccountsError = "Could not load account diagnostics."
		return
	}
	sync := PreferenceUtils{}.Object(PreferenceUtils{}.Object(answer)["sync"])
	cursor, _ := sync["cursorVersion"].(float64)
	attempts, _ := sync["attempts"].(float64)
	service.diagnostics[id] = AccountDiagnostics{
		CursorVersion: cursor, Attempts: attempts,
		NextRunAt: cmp.Or(DateLabelUtils{}.DateTime(PreferenceUtils{}.Text(sync, "nextRunAt"), service.timeZoneOffset), "Not scheduled"),
		LastCode:  PreferenceUtils{}.Text(sync, "lastCode"),
		UpdatedAt: DateLabelUtils{}.DateTime(PreferenceUtils{}.Text(sync, "scheduleUpdatedAt"), service.timeZoneOffset),
	}
}

func (service *ConnectedAccountsService) row(account ExternalAccount) ExternalAccountRow {
	diagnostics, loaded := service.diagnostics[account.ID]
	return ExternalAccountRow{
		ID: account.ID, MailboxAddress: account.MailboxAddress, Provider: account.Provider, State: account.State,
		Label: account.Label, Color: account.Color, Failure: account.Failure,
		CanPause: ExternalAccountUtils{}.CanPause(account.State), CanResume: account.State == "paused",
		CanReconnect: ExternalAccountUtils{}.CanReconnect(account.State), CanRetry: account.State == "degraded",
		CanRemove: account.State != "deleting", ProgressLabel: ExternalAccountUtils{}.ProgressLabel(account),
		StateLabel: ExternalAccountUtils{}.StateLabel(account.State), ReconnectUsername: service.ReconnectUsername,
		IsOAuth:          slices.Contains([]string{service.names().Microsoft, service.names().Gmail}, account.Provider),
		IsSharedMailbox:  FlowUtils{}.Both(account.Provider == service.names().Microsoft, account.SharedMailboxAddress != ""),
		DelegateIdentity: FlowUtils{}.Text(account.AuthIdentity != account.MailboxAddress, account.AuthIdentity, ""),
		CanRead:          account.CanReadMail, CanSend: account.CanSend,
		Reconnecting: service.reconnectID == account.ID, ConfirmingRemoval: service.removeID == account.ID,
		ShowingDiagnostics: service.diagnosticsID == account.ID, Mutating: service.mutationID == account.ID,
		Diagnostics: map[bool]*AccountDiagnostics{true: &diagnostics, false: nil}[loaded],
	}
}

func (service *ConnectedAccountsService) account(id string) ExternalAccount {
	index := slices.IndexFunc(service.Accounts, func(account ExternalAccount) bool { return account.ID == id })
	return append(slices.Clone(service.Accounts), ExternalAccount{})[cmp.Or(index+1, len(service.Accounts)+1)-1]
}

func (service *ConnectedAccountsService) provider(id string) MailProvider {
	index := slices.IndexFunc(service.Providers, func(provider MailProvider) bool { return provider.ID == id })
	return append(slices.Clone(service.Providers), MailProvider{})[cmp.Or(index+1, len(service.Providers)+1)-1]
}

func (service *ConnectedAccountsService) providerName(provider string) string {
	return FlowUtils{}.Text(provider == service.names().Gmail, "Google", "Microsoft")
}

func (service *ConnectedAccountsService) emptyDraft(address string) map[string]string {
	return service.draft(DiscoveredSettings{}, address)
}

func (service *ConnectedAccountsService) draft(settings DiscoveredSettings, address string) map[string]string {
	draft := map[string]string{}
	PreferenceUtils{}.Decode(ExternalAccountUtils{}.Draft(settings, address), &draft)
	return draft
}

func (service *ConnectedAccountsService) revisionBody(account ExternalAccount) BehaviorValue {
	return service.withRevision(account, map[string]any{})
}

// share keeps the accounts and tells other sections, such as new-mail alerts, about them.
func (service *ConnectedAccountsService) share(accounts []ExternalAccount) {
	service.Accounts = accounts
	service.applicationEventPublisher.PublishEvent(ConnectedAccountList{Accounts: accounts}, true)
}

func (service *ConnectedAccountsService) toast(message string) {
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (service *ConnectedAccountsService) complete(address string) bool {
	return strings.Contains(address, "@")
}

func (service *ConnectedAccountsService) defaultColor(provider string) string {
	return FlowUtils{}.Text(provider == service.names().Gmail, "#EA4335", "#0078D4")
}

func (service *ConnectedAccountsService) preparing(provider string) string {
	return "Preparing a secure " + service.providerName(provider) + " sign-in…"
}

func (service *ConnectedAccountsService) credentials(username string, password string) map[string]any {
	return map[string]any{service.names().Username: username, "password": password}
}

func (service *ConnectedAccountsService) withRevision(account ExternalAccount, body map[string]any) map[string]any {
	body["revision"] = account.Revision
	return body
}

func (service *ConnectedAccountsService) names() ConnectionNames {
	return ConnectionNames{
		Section: "connected-accounts",
		Address: "address",
		OAuth: "oauth",
		Gmail: "gmail",
		Microsoft: "microsoft",
		Choice: "choiceId",
		Shared: "sharedMailboxAddress",
		Label: "oauthLabel",
		Color: "oauthColor",
		Username: "username",
		Credentials: "credentials",
		Insecure: "allowInsecureTLS",
		CardDirectory: "carddav",
		CalendarDirectory: "caldav",
		Provider: "provider",
	}
}
