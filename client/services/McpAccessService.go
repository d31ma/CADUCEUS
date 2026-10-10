import (
	"cmp"
	"maps"
	"net/url"
	"slices"
	"strings"
)

// McpAccessService is the "AI / MCP access" section: whether MCP clients may
// connect, at which access level, to which mailbox domains and folders, the
// one-time bearer token, and the redacted activity log.
// @Service
type McpAccessService struct {
	// @Needs
	mcpClient *McpClient
	// @Needs
	clipboard                 Clipboard
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	accounts                  []CategoryAccount
	names                     map[string]string
	origin                    string
	timeZoneOffset            float64
	copied                    string
	Enabled                   bool
	AccessLevel               string
	AllowedDomains            []string
	ExcludedFolders           map[string][]string
	HasToken                  bool
	Token                     string
	TokenCreatedAt            string
	endpointPath              string
	Audit                     []McpAuditEvent
	Loading                   bool
	Saving                    bool
	Error                     string
}

// @OnStart
func (service *McpAccessService) Start() {
	service.Loading = true
	service.AccessLevel = service.levels()[0]
	service.AllowedDomains = []string{}
	service.ExcludedFolders = map[string][]string{}
	service.Audit = []McpAuditEvent{}
	service.endpointPath = service.defaultPath()
}

// @OnEvent({"replay":true})
func (service *McpAccessService) Catalog(catalog CategoryCatalog) {
	service.accounts = catalog.Accounts
}

// @OnEvent({"replay":true})
func (service *McpAccessService) Load(preferences UserPreferences) {
	service.names = MailboxUtils{}.Names(preferences.Values)
}

// @OnEvent({"replay":true})
func (service *McpAccessService) Learn(platform DevicePlatform) {
	service.origin = platform.Origin
	service.timeZoneOffset = platform.TimeZoneOffset
}

// @OnEvent({"replay":true})
func (service *McpAccessService) Open(session AccountSession) error {
	service.authorization = "Bearer " + session.Token
	config, configFailure := service.mcpClient.Config(service.authorization)
	audit, auditFailure := service.mcpClient.Audit(service.authorization)
	FlowUtils{}.When(configFailure == nil, func() { service.applyConfig(PreferenceUtils{}.Object(config)) })
	service.Audit = append([]McpAuditEvent{}, audit...)
	FlowUtils{}.When(auditFailure != nil, func() { service.Audit = []McpAuditEvent{} })
	service.Loading = false
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "mcp"}, false)
	return nil
}

func (service *McpAccessService) ToggleEnabled() {
	service.Enabled = !service.Enabled
}

func (service *McpAccessService) SetAccessLevel(level string) {
	service.AccessLevel = PreferenceUtils{}.Choice(level, service.levels(), service.AccessLevel)
}

// ToggleDomain exposes or hides one mailbox; hiding it forgets its folder exclusions.
func (service *McpAccessService) ToggleDomain(domain string) {
	service.AllowedDomains = PreferenceUtils{}.Toggle(service.AllowedDomains, domain)
	excluded := maps.Clone(service.ExcludedFolders)
	FlowUtils{}.When(!slices.Contains(service.AllowedDomains, domain), func() { delete(excluded, domain) })
	service.ExcludedFolders = excluded
}

func (service *McpAccessService) ToggleFolder(domain string, path string) {
	excluded := maps.Clone(service.ExcludedFolders)
	excluded[domain] = PreferenceUtils{}.Toggle(excluded[domain], path)
	maps.DeleteFunc(excluded, func(domain string, folders []string) bool { return len(folders) == 0 })
	service.ExcludedFolders = excluded
}

func (service *McpAccessService) Save() error {
	FlowUtils{}.When(!service.Saving, service.save)
	return nil
}

func (service *McpAccessService) RotateToken() error {
	FlowUtils{}.When(!service.Saving, service.rotate)
	return nil
}

func (service *McpAccessService) RevokeToken() error {
	FlowUtils{}.When(!service.Saving, service.revoke)
	return nil
}

func (service *McpAccessService) ClearAudit() error {
	FlowUtils{}.When(!service.Saving, service.clear)
	return nil
}

// Copy puts the endpoint or token on the clipboard.
func (service *McpAccessService) Copy(value string, label string) {
	service.copied = label
	problem := service.clipboard.WriteText(value)
	FlowUtils{}.Choose(problem == nil, func() { service.toast(label + " copied.") }, service.copyFailed)
}

// @OnEvent
func (service *McpAccessService) Refuse(failed BrowserCallFailed) {
	FlowUtils{}.When(failed.Api == "clipboard", service.copyFailed)
}

func (service *McpAccessService) DomainRows() []McpDomainRow {
	return service.domainRows(service.accounts)
}

func (service *McpAccessService) FolderRows() []McpFolderRow {
	return service.folderRows(service.accounts)
}

func (service *McpAccessService) AuditRows() []McpAuditRow {
	return service.auditRows()
}

// Endpoint is the full address that an MCP client connects to.
func (service *McpAccessService) Endpoint() string {
	origin := service.parsed(service.origin)
	return origin.ResolveReference(service.parsed(cmp.Or(service.endpointPath, service.defaultPath()))).String()
}

func (service *McpAccessService) TokenCreatedAtLabel() string {
	return DateLabelUtils{}.DateTime(service.TokenCreatedAt, service.timeZoneOffset)
}

func (service *McpAccessService) applyConfig(config map[string]any) {
	service.Enabled = PreferenceUtils{}.Flag(config, service.enabledKey())
	service.AccessLevel = PreferenceUtils{}.Choice(PreferenceUtils{}.Text(config, "accessLevel"), service.levels(), service.levels()[0])
	service.AllowedDomains = PreferenceUtils{}.Strings(config, service.domainsKey())
	excluded := map[string][]string{}
	PreferenceUtils{}.Decode(config[service.foldersKey()], &excluded)
	service.ExcludedFolders = excluded
	service.HasToken = PreferenceUtils{}.Flag(config, "hasToken")
	service.TokenCreatedAt = PreferenceUtils{}.Text(config, "tokenCreatedAt")
	service.endpointPath = cmp.Or(PreferenceUtils{}.Text(config, "endpoint"), service.defaultPath())
}

func (service *McpAccessService) save() {
	service.Saving = true
	service.Error = ""
	body := map[string]any{service.enabledKey(): service.Enabled, "accessLevel": service.AccessLevel, service.domainsKey(): service.AllowedDomains, service.foldersKey(): service.ExcludedFolders}
	stored, failure := service.mcpClient.SaveConfig(service.authorization, body)
	service.Saving = false
	service.showSaved(PreferenceUtils{}.Object(stored), failure)
}

func (service *McpAccessService) showSaved(stored map[string]any, failure error) {
	if failure != nil {
		service.Error = "Could not save MCP access."
		return
	}
	service.Enabled = PreferenceUtils{}.Flag(stored, service.enabledKey())
	service.AllowedDomains = PreferenceUtils{}.Strings(stored, service.domainsKey())
	excluded := map[string][]string{}
	PreferenceUtils{}.Decode(stored[service.foldersKey()], &excluded)
	service.ExcludedFolders = excluded
	service.toast("MCP access saved.")
}

func (service *McpAccessService) rotate() {
	service.Saving = true
	service.Error = ""
	token, failure := service.mcpClient.RotateToken(service.authorization)
	service.Saving = false
	service.showRotated(token, failure)
}

func (service *McpAccessService) showRotated(token McpToken, failure error) {
	if failure != nil {
		service.Error = "Could not generate the MCP token."
		return
	}
	service.Token = token.Token
	service.HasToken = true
	service.TokenCreatedAt = token.CreatedAt
	service.toast("MCP token rotated. Copy it now.")
}

func (service *McpAccessService) revoke() {
	service.Saving = true
	service.Error = ""
	revocation, failure := service.mcpClient.RevokeToken(service.authorization)
	service.Saving = false
	service.showRevoked(revocation, failure)
}

func (service *McpAccessService) showRevoked(revocation McpRevocation, failure error) {
	if failure != nil {
		service.Error = "Could not revoke the MCP token."
		return
	}
	service.Token = ""
	service.HasToken = false
	service.TokenCreatedAt = ""
	service.toast(FlowUtils{}.Text(revocation.Revoked, "MCP token revoked.", "No active MCP token."))
}

func (service *McpAccessService) clear() {
	service.Saving = true
	_, failure := service.mcpClient.ClearAudit(service.authorization)
	service.Saving = false
	FlowUtils{}.Choose(failure == nil, func() {
		service.Audit = []McpAuditEvent{}
		service.toast("MCP activity cleared.")
	}, func() { service.Error = "Could not clear MCP activity." })
}

func (service *McpAccessService) copyFailed() {
	service.Error = "Could not copy " + strings.ToLower(service.copied) + "."
}

func (service *McpAccessService) domainRows(accounts []CategoryAccount) []McpDomainRow {
	rows := []McpDomainRow{}
	for _, account := range accounts {
		rows = append(rows, McpDomainRow{Domain: account.Domain, Label: MailboxUtils{}.Label(service.names, account.Domain), Allowed: slices.Contains(service.AllowedDomains, account.Domain)})
	}
	return rows
}

func (service *McpAccessService) folderRows(accounts []CategoryAccount) []McpFolderRow {
	rows := []McpFolderRow{}
	for _, account := range accounts {
		rows = append(rows, service.accountFolders(account)...)
	}
	return rows
}

func (service *McpAccessService) accountFolders(account CategoryAccount) []McpFolderRow {
	rows := []McpFolderRow{}
	allowed := slices.Contains(service.AllowedDomains, account.Domain)
	for _, category := range account.Categories {
		rows = append(rows, McpFolderRow{
			Domain: account.Domain, Path: category.Path, Role: category.Role, AccountAllowed: allowed,
			Excluded: slices.Contains(service.ExcludedFolders[account.Domain], category.Path),
		})
	}
	return rows
}

func (service *McpAccessService) auditRows() []McpAuditRow {
	rows := []McpAuditRow{}
	for _, event := range service.Audit {
		rows = append(rows, McpAuditRow{
			ToolName: event.ToolName, Status: event.Status, DurationMs: event.DurationMs, ResultCount: event.ResultCount,
			CreatedAtLabel: DateLabelUtils{}.DateTime(event.CreatedAt, service.timeZoneOffset),
			ParameterLabel: cmp.Or(strings.Join(event.ParameterKeys, ", "), "none"),
		})
	}
	return rows
}

func (service *McpAccessService) parsed(address string) *url.URL {
	parsed, failure := url.Parse(address)
	if failure != nil {
		return &url.URL{}
	}
	return parsed
}

func (service *McpAccessService) levels() []string {
	return []string{"read-only", "read-write", "read-write-send"}
}

func (service *McpAccessService) enabledKey() string { return "enabled" }

func (service *McpAccessService) defaultPath() string { return "/mcp" }

func (service *McpAccessService) domainsKey() string { return "allowedDomains" }

func (service *McpAccessService) foldersKey() string { return "excludedFolders" }

func (service *McpAccessService) toast(message string) {
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}
