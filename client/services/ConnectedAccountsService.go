import (
	"cmp"
	"errors"
	"slices"
	"strconv"
	"strings"
)

// ConnectedAccountsService is the "Connected accounts" section: outside
// mailboxes, the guided first-run setup, provider discovery, password and
// OAuth connection, and the pause, resume, retry, reconnect and remove
// actions. Passwords live only in memory until the request that uses them.
// The provider's OAuth redirect comes back to this page: the service reads
// it from PageLocation, removes its secrets from the address with
// AddressReplaced and keeps the onboarding flags in storage.
// @Service
type ConnectedAccountsService struct {
	// @Needs
	externalAccountsClient *ExternalAccountsClient
	// @Needs
	mailProvidersClient       *MailProvidersClient
	applicationEventPublisher ApplicationEventPublisher
	timeZoneOffset            float64
	authorization             string
	signedIn                  bool
	owner                     string
	location                  *PageLocation
	stored                    *StoredValues
	launched                  bool
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
	service.Accounts = []ExternalAccount{}
	service.Providers = []MailProvider{}
	service.SetupStep = cmp.Or(service.SetupStep, service.names().Address)
	FlowUtils{}.When(service.Draft == nil, func() { service.Draft = service.emptyDraft("") })()
}

// @OnEvent({"replay":true})
func (service *ConnectedAccountsService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
	service.signedIn = changed.SignedIn
	service.owner = cmp.Or(changed.Email, "owner")
	refused := false
	FlowUtils{}.When(changed.SignedIn, func() { refused = service.load() })()
	service.resume()
	service.announce(refused)
}

// @OnEvent({"replay":true})
func (service *ConnectedAccountsService) Learn(platform DevicePlatform) {
	service.timeZoneOffset = platform.TimeZoneOffset
}

// @OnEvent({"replay":true})
func (service *ConnectedAccountsService) HearLocation(location PageLocation) {
	service.location = &location
	service.arrive()
}

// @OnEvent({"replay":true})
func (service *ConnectedAccountsService) HearStorage(stored StoredValues) {
	service.stored = &stored
	service.arrive()
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
	service.remember(map[string]string{ConnectedAccountUtils{}.Names().Pending: service.flag()}, []string{})
	service.OpenSetup()
}

func (service *ConnectedAccountsService) CloseSetupOrOnboarding() {
	FlowUtils{}.Either(service.Onboarding, service.dismissOnboarding, service.closeSetup)()
}

// SelectProvider moves the setup to the OAuth or credentials step of one catalog entry.
func (service *ConnectedAccountsService) SelectProvider(id string) {
	provider := service.provider(id)
	FlowUtils{}.Either(provider.Authentication == service.names().OAuth, func() { service.selectOAuth(provider) }, func() { service.selectCredentials(provider) })()
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
	FlowUtils{}.Either(service.complete(address), func() { service.discover(address) }, func() { service.SetupError = "Enter a complete email address." })()
	return nil
}

// BeginOAuth starts provider authorization for the mailbox in the OAuth step.
func (service *ConnectedAccountsService) BeginOAuth() error {
	identity := PreferenceUtils{}.Lower(service.Draft["authIdentity"])
	shared := PreferenceUtils{}.Lower(service.Draft[service.names().Shared])
	complete := ChoiceUtils{}.All(service.complete(identity), ChoiceUtils{}.Any(shared == "", service.complete(shared)))
	FlowUtils{}.Either(complete, func() { service.beginNew(identity, shared) }, func() {
		service.SetupError = "Enter a complete provider sign-in address and, when used, a complete shared mailbox address."
	})()
	return nil
}

// ValidateAndCreate checks the servers without sending mail, then saves the account.
func (service *ConnectedAccountsService) ValidateAndCreate() error {
	request, problem := service.credentialsRequest()
	service.Password = ""
	FlowUtils{}.Either(problem == nil, func() { service.create(request) }, func() { service.SetupError = problem.Error() })()
	return nil
}

// Mutate pauses or resumes one account.
func (service *ConnectedAccountsService) Mutate(id string, action string) error {
	account := service.account(id)
	answer, failure := service.externalAccountsClient.Update(service.authorization, id, service.withRevision(account, map[string]any{"action": action}))
	service.showReplaced(answer, failure, "Could not "+action+" this account.")
	return nil
}

func (service *ConnectedAccountsService) Retry(id string) error {
	account := service.account(id)
	answer, failure := service.externalAccountsClient.Retry(service.authorization, id, service.revisionBody(account))
	FlowUtils{}.Either(ChoiceUtils{}.All(failure == nil, answer.Scheduled), func() { service.retried(id) }, func() {
		service.AccountsError = HTTPUtils{}.Message(failure, "Could not schedule an account retry.", service.network())
	})()
	return nil
}

// ToggleDiagnostics opens or closes the redacted synchronization details.
func (service *ConnectedAccountsService) ToggleDiagnostics(id string) error {
	FlowUtils{}.Either(service.diagnosticsID == id, func() { service.diagnosticsID = "" }, func() { service.loadDiagnostics(id) })()
	return nil
}

func (service *ConnectedAccountsService) OpenReconnect(id string) {
	account := service.account(id)
	service.reconnectID = id
	service.removeID = ""
	service.ReconnectUsername = cmp.Or(account.Connection.Imap.Username, account.MailboxAddress)
	service.ReconnectPassword = ""
}

func (service *ConnectedAccountsService) CancelReconnect() {
	service.reconnectID = ""
	service.ReconnectPassword = ""
}

// Reconnect authorizes an OAuth account again or replaces a password.
func (service *ConnectedAccountsService) Reconnect(id string) error {
	account := service.account(id)
	FlowUtils{}.Either(ConnectedAccountUtils{}.OAuth(account.Provider), func() { service.beginReconnect(account) }, func() { service.reconnectPassword(account) })()
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
	FlowUtils{}.Either(failure == nil, func() { service.markRemoving(id) }, func() {
		service.failMutation(HTTPUtils{}.Message(failure, "Could not remove this account.", service.network()))
	})()
	return nil
}

// Rows are the account cards, with the state of their inline panels.
func (service *ConnectedAccountsService) Rows() []ExternalAccountRow {
	return service.rows()
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

func (service *ConnectedAccountsService) rows() []ExternalAccountRow {
	rows := []ExternalAccountRow{}
	for _, account := range service.Accounts {
		rows = append(rows, service.row(account))
	}
	return rows
}

// load reads the accounts and the provider catalog, and says whether the
// server refused either read.
func (service *ConnectedAccountsService) load() bool {
	accounts, accountsFailure := service.externalAccountsClient.Accounts(service.authorization)
	catalog, catalogFailure := service.mailProvidersClient.Catalog(service.authorization)
	failure := errors.Join(accountsFailure, catalogFailure)
	service.AccountsError = ""
	FlowUtils{}.Either(failure == nil, func() { service.showLoaded(accounts, catalog) }, func() {
		service.AccountsError = HTTPUtils{}.Message(failure, "Could not load connected accounts.", service.network())
	})()
	return ChoiceUtils{}.Any(HTTPUtils{}.Refused(accountsFailure), HTTPUtils{}.Refused(catalogFailure))
}

func (service *ConnectedAccountsService) showLoaded(accounts ExternalAccountList, catalog ProviderCatalog) {
	service.share(ExternalAccountUtils{}.Normalize(accounts.Accounts))
	service.Providers = ConnectedAccountUtils{}.Providers(catalog.Providers)
}

// arrive reads the launch once both the address and the stored keys came.
func (service *ConnectedAccountsService) arrive() {
	ready := ChoiceUtils{}.All(service.location != nil, service.stored != nil, !service.launched)
	FlowUtils{}.When(ready, service.begin)()
}

// begin cleans the provider's secrets out of the address and the storage,
// opens the setup that the address asked for, and finishes a redirect.
func (service *ConnectedAccountsService) begin() {
	service.launched = true
	service.launch = ConnectedAccountUtils{}.Launch(*service.location, *service.stored)
	current := service.location.Path + service.location.Search + service.location.Hash
	FlowUtils{}.When(service.launch.Cleaned != current, func() {
		service.applicationEventPublisher.PublishEvent(AddressReplaced{Target: service.launch.Cleaned}, false)
	})()
	service.forgetProvider()
	FlowUtils{}.When(service.launch.Onboarding, service.StartOnboarding)()
	service.resume()
	service.announce(false)
}

func (service *ConnectedAccountsService) forgetProvider() {
	key := ConnectedAccountUtils{}.Names().ProviderKey
	service.applicationEventPublisher.PublishEvent(StorageWritten{Area: service.tab(), Values: map[string]string{}, Removed: []string{key}}, false)
	service.remember(map[string]string{}, []string{service.keyed(key, service.launch.State)})
}

// resume finishes a provider redirect once both the session and the
// callback parameters have arrived, in whichever order they came.
func (service *ConnectedAccountsService) resume() {
	ready := ChoiceUtils{}.All(service.signedIn, service.launch.HasCallback, !service.callbackHandled)
	FlowUtils{}.When(ready, service.finishAuthorization)()
}

func (service *ConnectedAccountsService) finishAuthorization() {
	service.callbackHandled = true
	FlowUtils{}.Either(service.launch.Failed, func() {
		service.OAuthNotice = "Provider authorization was not completed. No account credentials were changed."
	}, service.completeAuthorization)()
}

func (service *ConnectedAccountsService) completeAuthorization() {
	body := map[string]any{"code": service.launch.Code, "state": service.launch.State}
	answer, failure := service.completeWith(cmp.Or(service.launch.Provider, service.names().Microsoft), body)
	answer, failure = service.completeWithGoogle(answer, failure, body)
	FlowUtils{}.Either(failure == nil, func() { service.showAuthorized(answer) }, func() {
		service.OAuthNotice = "Provider authorization could not be completed. Try reconnecting the account."
	})()
}

func (service *ConnectedAccountsService) completeWith(provider string, body BehaviorValue) (ExternalAccount, error) {
	if provider == service.names().Gmail {
		return service.externalAccountsClient.CompleteGoogle(service.authorization, body)
	}
	return service.externalAccountsClient.CompleteMicrosoft(service.authorization, body)
}

// completeWithGoogle tries Google after Microsoft when no provider was remembered.
func (service *ConnectedAccountsService) completeWithGoogle(answer ExternalAccount, failure error, body BehaviorValue) (ExternalAccount, error) {
	if failure != nil && service.launch.Provider == "" {
		return service.externalAccountsClient.CompleteGoogle(service.authorization, body)
	}
	return answer, failure
}

func (service *ConnectedAccountsService) showAuthorized(answer ExternalAccount) {
	kind := ChoiceUtils{}.Text(answer.Provider == service.names().Gmail, "Google account", "Microsoft account")
	service.OAuthNotice = ChoiceUtils{}.Text(answer.SharedMailboxAddress != "", "Shared mailbox", kind) + " connected successfully."
	service.completeOnboarding()
	service.toast(service.OAuthNotice)
	service.load()
	service.refreshMail()
}

func (service *ConnectedAccountsService) dismissOnboarding() {
	service.Onboarding = false
	service.finishOnboarding()
	service.closeSetup()
}

func (service *ConnectedAccountsService) completeOnboarding() {
	FlowUtils{}.When(service.Onboarding, func() {
		service.Onboarding = false
		service.SetupOpen = false
		service.finishOnboarding()
	})()
}

// finishOnboarding records that this owner finished or dismissed the
// first-run account setup, so no page offers it again.
func (service *ConnectedAccountsService) finishOnboarding() {
	names := ConnectedAccountUtils{}.Names()
	service.remember(map[string]string{service.keyed(names.Completed, service.owner): service.flag()}, []string{names.Pending})
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
	FlowUtils{}.Either(provider.Configured, func() { service.enterOAuth(provider) }, func() {
		service.SetupError = provider.Name + " OAuth must be configured by this server’s administrator before this account can be connected."
	})()
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
	service.SetupStep = service.names().Credentials
	service.Draft = ConnectedAccountUtils{}.Draft(ConnectedAccountUtils{}.PresetSettings(provider), service.Address)
}

func (service *ConnectedAccountsService) discover(address string) {
	service.Address = address
	service.SetupBusy = true
	service.SetupError = ""
	service.SetupStatus = "Discovering secure mail settings…"
	settings, failure := service.mailProvidersClient.Discover(service.authorization, map[string]any{service.names().Address: address})
	service.SetupBusy = false
	FlowUtils{}.Either(failure == nil, func() { service.showDiscovered(settings, address) }, func() {
		service.SetupError = HTTPUtils{}.Message(failure, "Mail settings could not be discovered.", service.network())
		service.SetupStatus = ""
	})()
}

func (service *ConnectedAccountsService) showDiscovered(settings DiscoveredSettings, address string) {
	provider := service.provider(settings.ChoiceID)
	FlowUtils{}.Either(provider.Authentication == service.names().OAuth, func() { service.discoveredOAuth(provider) }, func() { service.discoveredServers(settings, address) })()
}

func (service *ConnectedAccountsService) discoveredOAuth(provider MailProvider) {
	service.selectOAuth(provider)
	entered := service.SetupStep == service.names().OAuth
	service.SetupError = ChoiceUtils{}.Text(entered, "", cmp.Or(service.SetupError, provider.Name+" OAuth is not available."))
	service.SetupStatus = ChoiceUtils{}.Text(entered, provider.Name+" settings found. Confirm the sign-in address to continue.", "")
}

func (service *ConnectedAccountsService) discoveredServers(settings DiscoveredSettings, address string) {
	service.Draft = ConnectedAccountUtils{}.Draft(settings, address)
	service.SetupStep = service.names().Credentials
	service.SetupStatus = ChoiceUtils{}.Text(settings.Source == "fallback",
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
	authorization, problem := service.authorize(provider, request)
	service.showRedirect(provider, authorization, problem, func(message string) { service.SetupError = message })
}

func (service *ConnectedAccountsService) beginReconnect(account ExternalAccount) {
	service.mutationID = account.ID
	service.AccountsError = ""
	service.SetupStatus = service.preparing(account.Provider)
	request := service.withRevision(account, map[string]any{service.names().Provider: account.Provider, "accountId": account.ID})
	authorization, problem := service.authorize(account.Provider, request)
	service.showRedirect(account.Provider, authorization, problem, func(message string) { service.AccountsError = message })
}

// authorize asks the server for the provider's sign-in address and checks
// that it really is the provider's own HTTPS host.
func (service *ConnectedAccountsService) authorize(provider string, request map[string]any) (ProviderAuthorization, error) {
	authorization, failure := service.beginWith(provider, request)
	trusted := ConnectedAccountUtils{}.Trusted(provider, authorization.AuthorizationURL)
	return authorization, cmp.Or(service.startFailure(failure), service.addressFailure(trusted))
}

func (service *ConnectedAccountsService) beginWith(provider string, request map[string]any) (ProviderAuthorization, error) {
	if provider == service.names().Gmail {
		return service.externalAccountsClient.BeginGoogle(service.authorization, request)
	}
	return service.externalAccountsClient.BeginMicrosoft(service.authorization, request)
}

func (service *ConnectedAccountsService) startFailure(failure error) error {
	if failure != nil {
		return errors.New(HTTPUtils{}.Message(failure, "Provider authorization could not be started.", service.network()))
	}
	return nil
}

func (service *ConnectedAccountsService) addressFailure(trusted bool) error {
	if !trusted {
		return errors.New("The server returned an invalid provider authorization address.")
	}
	return nil
}

func (service *ConnectedAccountsService) showRedirect(provider string, authorization ProviderAuthorization, problem error, report func(string)) {
	if problem != nil {
		report(problem.Error())
		service.SetupStatus = ""
		service.SetupBusy = false
		service.mutationID = ""
		return
	}
	service.leave(provider, authorization)
}

// leave remembers which provider the redirect belongs to and opens the
// provider's sign-in page.
func (service *ConnectedAccountsService) leave(provider string, authorization ProviderAuthorization) {
	names := ConnectedAccountUtils{}.Names()
	pending := map[bool]map[string]string{true: {names.Pending: service.flag()}, false: {}}[service.Onboarding]
	local := AppearanceUtils{}.Merged(pending, map[string]string{service.keyed(names.ProviderKey, authorization.State): provider})
	service.applicationEventPublisher.PublishEvent(StorageWritten{Area: service.tab(), Values: map[string]string{names.ProviderKey: provider}, Removed: []string{}}, false)
	service.remember(local, []string{})
	service.applicationEventPublisher.PublishEvent(NavigationRequested{Target: authorization.AuthorizationURL}, false)
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
		"imap":                      service.endpoint(imapHost, imapPort, draft["imapSecurity"], username),
		"smtp":                      service.endpoint(smtpHost, smtpPort, draft["smtpSecurity"], username),
	}
	service.addDirectory(request, service.names().CardDirectory, draft["cardDAVURL"], cmp.Or(strings.TrimSpace(draft["cardDAVUsername"]), username))
	service.addDirectory(request, service.names().CalendarDirectory, draft["calDAVURL"], cmp.Or(strings.TrimSpace(draft["calDAVUsername"]), username))
	missing := slices.Contains([]string{mailbox, username, service.Password}, "")
	servers := ChoiceUtils{}.All(imapHost != "", smtpHost != "", errors.Join(imapFailure, smtpFailure) == nil)
	return request, service.requestProblem(missing, servers)
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
	FlowUtils{}.Either(creationFailure == nil, func() { service.showCreated(created) }, func() {
		service.SetupError = ChoiceUtils{}.Text(validationFailure != nil, "The mail servers rejected these settings.", "The account could not be added.")
		service.SetupStatus = ""
	})()
}

func (service *ConnectedAccountsService) validatingStatus(request map[string]any) string {
	directories := slices.DeleteFunc([]string{
		ChoiceUtils{}.Text(request[service.names().CardDirectory] != nil, "CardDAV", ""),
		ChoiceUtils{}.Text(request[service.names().CalendarDirectory] != nil, "CalDAV", ""),
	}, func(name string) bool { return name == "" })
	return ChoiceUtils{}.Text(len(directories) > 0, "Validating IMAP, SMTP, and "+strings.Join(directories, " and ")+" without sending mail…", "Validating IMAP and SMTP without sending mail…")
}

func (service *ConnectedAccountsService) saveValidated(request map[string]any, validationFailure error) (ExternalAccount, error) {
	if validationFailure != nil {
		return ExternalAccount{}, validationFailure
	}
	service.SetupStatus = "Validation passed. Saving the encrypted account credential…"
	return service.externalAccountsClient.Create(service.authorization, request)
}

func (service *ConnectedAccountsService) showCreated(created ExternalAccount) {
	service.share(ExternalAccountUtils{}.Normalize(append(slices.Clone(service.Accounts), created)))
	service.completeOnboarding()
	service.closeSetup()
	service.toast("External account connected.")
	service.refreshMail()
}

func (service *ConnectedAccountsService) reconnectPassword(account ExternalAccount) {
	password := service.ReconnectPassword
	service.ReconnectPassword = ""
	FlowUtils{}.When(password != "", func() { service.sendReconnect(account, password) })()
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
	})()
}

func (service *ConnectedAccountsService) showReplaced(answer ExternalAccount, failure error, message string) {
	service.mutationID = ""
	if failure != nil {
		service.failMutation(HTTPUtils{}.Message(failure, message, service.network()))
		return
	}
	service.AccountsError = ""
	service.share(ExternalAccountUtils{}.Replaced(service.Accounts, []ExternalAccount{answer}))
}

func (service *ConnectedAccountsService) failMutation(message string) {
	service.mutationID = ""
	service.load()
	service.AccountsError = message
}

func (service *ConnectedAccountsService) markRemoving(id string) {
	removing := slices.Clone(service.Accounts)
	for index := range removing {
		removing[index] = service.removing(removing[index], id)
	}
	service.share(removing)
	service.removeID = ""
	service.toast("Account removal started.")
}

// removing marks the account being removed; the others stay as they are.
func (service *ConnectedAccountsService) removing(account ExternalAccount, id string) ExternalAccount {
	if account.ID != id {
		return account
	}
	account.State = "deleting"
	account.Revision += 1
	account.Progress = nil
	account.Failure = nil
	return account
}

func (service *ConnectedAccountsService) retried(id string) {
	service.toast("Account retry scheduled.")
	service.loadDiagnostics(id)
}

func (service *ConnectedAccountsService) loadDiagnostics(id string) {
	service.diagnosticsID = id
	report, failure := service.externalAccountsClient.Diagnostics(service.authorization, id)
	FlowUtils{}.Either(failure == nil, func() { service.diagnostics[id] = ConnectedAccountUtils{}.Diagnostics(report, service.timeZoneOffset) }, func() {
		service.AccountsError = HTTPUtils{}.Message(failure, "Could not load account diagnostics.", service.network())
	})()
}

func (service *ConnectedAccountsService) row(account ExternalAccount) ExternalAccountRow {
	diagnostics, loaded := service.diagnostics[account.ID]
	return ExternalAccountRow{
		ID: account.ID, MailboxAddress: account.MailboxAddress, Provider: account.Provider, State: account.State,
		Label: account.Label, Color: account.Color, Failure: account.Failure,
		CanPause: ConnectedAccountUtils{}.CanPause(account.State), CanResume: account.State == "paused",
		CanReconnect: ConnectedAccountUtils{}.CanReconnect(account.State), CanRetry: account.State == "degraded",
		CanRemove: account.State != "deleting", ProgressLabel: ConnectedAccountUtils{}.ProgressLabel(account),
		StateLabel: ConnectedAccountUtils{}.StateLabel(account.State), ReconnectUsername: service.ReconnectUsername,
		IsOAuth:          ConnectedAccountUtils{}.OAuth(account.Provider),
		IsSharedMailbox:  ChoiceUtils{}.All(account.Provider == service.names().Microsoft, account.SharedMailboxAddress != ""),
		DelegateIdentity: ChoiceUtils{}.Text(account.AuthIdentity != account.MailboxAddress, account.AuthIdentity, ""),
		CanRead:          account.Capabilities.MailRead, CanSend: account.Capabilities.Send,
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

func (service *ConnectedAccountsService) emptyDraft(address string) map[string]string {
	return ConnectedAccountUtils{}.Draft(DiscoveredSettings{}, address)
}

func (service *ConnectedAccountsService) revisionBody(account ExternalAccount) BehaviorValue {
	return service.withRevision(account, map[string]any{})
}

// share keeps the accounts and tells other sections, such as new-mail alerts, about them.
func (service *ConnectedAccountsService) share(accounts []ExternalAccount) {
	service.Accounts = accounts
	service.applicationEventPublisher.PublishEvent(ConnectedAccountList{Accounts: accounts}, true)
}

// refreshMail asks the shell to count the mailboxes again after an account
// was connected.
func (service *ConnectedAccountsService) refreshMail() {
	service.applicationEventPublisher.PublishEvent(InboxRefreshRequested{Reason: "external-account"}, false)
}

func (service *ConnectedAccountsService) remember(values map[string]string, removed []string) {
	service.applicationEventPublisher.PublishEvent(StorageWritten{Area: "local", Values: values, Removed: removed}, false)
}

func (service *ConnectedAccountsService) toast(message string) {
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (service *ConnectedAccountsService) announce(refused bool) {
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: service.names().Section, Refused: refused}, false)
}

func (service *ConnectedAccountsService) complete(address string) bool {
	return strings.Contains(address, "@")
}

func (service *ConnectedAccountsService) defaultColor(provider string) string {
	return ChoiceUtils{}.Text(provider == service.names().Gmail, "#EA4335", "#0078D4")
}

func (service *ConnectedAccountsService) preparing(provider string) string {
	return "Preparing a secure " + ChoiceUtils{}.Text(provider == service.names().Gmail, "Google", "Microsoft") + " sign-in…"
}

func (service *ConnectedAccountsService) credentials(username string, password string) map[string]any {
	return map[string]any{service.names().Username: username, "password": password}
}

func (service *ConnectedAccountsService) withRevision(account ExternalAccount, body map[string]any) map[string]any {
	body["revision"] = account.Revision
	return body
}

func (service *ConnectedAccountsService) network() string { return "Network error." }

// keyed is a storage key that belongs to one owner or one redirect.
func (service *ConnectedAccountsService) keyed(key string, owner string) string {
	return key + ":" + owner
}

// flag is the stored value of a yes flag.
func (service *ConnectedAccountsService) flag() string { return "1" }

// tab is the storage area that lives as long as this browser tab.
func (service *ConnectedAccountsService) tab() string { return "session" }

func (service *ConnectedAccountsService) names() ConnectionNames {
	return ConnectionNames{
		Section:           "connected-accounts",
		Address:           "address",
		OAuth:             "oauth",
		Gmail:             "gmail",
		Microsoft:         "microsoft",
		Choice:            "choiceId",
		Shared:            "sharedMailboxAddress",
		Label:             "oauthLabel",
		Color:             "oauthColor",
		Username:          "username",
		Credentials:       "credentials",
		Insecure:          "allowInsecureTLS",
		CardDirectory:     "carddav",
		CalendarDirectory: "caldav",
		Provider:          "provider",
	}
}
