import (
	"cmp"
	"slices"
)

// FolderRolesService maps each system role (inbox, sent, drafts, spam,
// archive, trash) to a physical folder of every mailbox, and saves the
// whole form in one request.
// @Service
type FolderRolesService struct {
	// @Needs
	categoriesClient *CategoriesClient
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	names                     map[string]string
	drafts                    map[string]string
	Accounts                  []CategoryAccount
	Saving                    bool
	Error                     string
}

// @OnStart
func (service *FolderRolesService) Start() {
	service.drafts = map[string]string{}
	service.names = map[string]string{}
	service.Accounts = []CategoryAccount{}
}

// @OnEvent({"replay":true})
func (service *FolderRolesService) Load(preferences UserPreferences) {
	service.names = preferences.MailboxNames
}

// @OnEvent({"replay":true})
func (service *FolderRolesService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
	refused := false
	FlowUtils{}.When(changed.SignedIn, func() { refused = service.reload() })()
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "folder-roles", Refused: refused}, false)
}

// Rows are one editable field per mailbox and role.
func (service *FolderRolesService) Rows() []CategoryRow {
	return service.rows()
}

// Paths are every physical folder path, offered as suggestions.
func (service *FolderRolesService) Paths() []string {
	paths := service.paths()
	slices.Sort(paths)
	return slices.Compact(paths)
}

func (service *FolderRolesService) Edit(domain string, role string, value string) {
	service.drafts[service.draftKey(domain, role)] = value
}

func (service *FolderRolesService) SaveAll() error {
	mappings := service.mappings()
	service.fill(mappings, service.Rows())
	service.save(mappings)
	return nil
}

// ResetAll maps every role to the folder of the same name.
func (service *FolderRolesService) ResetAll() error {
	mappings := service.mappings()
	service.fillDefaults(mappings)
	service.save(mappings)
	return nil
}

func (service *FolderRolesService) roles() []CategoryRole {
	return []CategoryRole{
		{ID: "inbox", Label: "Inbox"}, {ID: "sent", Label: "Sent mail"}, {ID: "drafts", Label: "Drafts"},
		{ID: "spam", Label: "Spam"}, {ID: "archive", Label: "Archive"}, {ID: "trash", Label: "Trash"},
	}
}

func (service *FolderRolesService) rows() []CategoryRow {
	roles := service.roles()
	rows := []CategoryRow{}
	for index := range len(service.Accounts) * len(roles) {
		account, role := service.Accounts[index/len(roles)], roles[index%len(roles)]
		rows = append(rows, CategoryRow{
			Domain: account.Domain, Role: role.ID, Label: role.Label,
			Mailbox: MailboxUtils{}.Label(service.names, account.Domain), Value: cmp.Or(account.Mappings[role.ID], role.ID),
		})
	}
	return rows
}

func (service *FolderRolesService) paths() []string {
	paths := []string{}
	for _, account := range service.Accounts {
		paths = append(paths, service.categoryPaths(account.Categories)...)
	}
	return paths
}

func (service *FolderRolesService) categoryPaths(categories []Category) []string {
	paths := []string{}
	for _, category := range categories {
		paths = append(paths, category.Path)
	}
	return paths
}

func (service *FolderRolesService) mappings() map[string]map[string]string {
	mappings := map[string]map[string]string{}
	for _, account := range service.Accounts {
		mappings[account.Domain] = map[string]string{}
	}
	return mappings
}

// fill keeps an edited field, and otherwise the folder the field showed.
func (service *FolderRolesService) fill(mappings map[string]map[string]string, rows []CategoryRow) {
	for _, row := range rows {
		edited, present := service.drafts[service.draftKey(row.Domain, row.Role)]
		mappings[row.Domain][row.Role] = ChoiceUtils{}.Text(present, edited, row.Value)
	}
}

func (service *FolderRolesService) fillDefaults(mappings map[string]map[string]string) {
	for _, row := range service.Rows() {
		mappings[row.Domain][row.Role] = row.Role
	}
}

func (service *FolderRolesService) save(mappings map[string]map[string]string) {
	service.Saving = true
	service.Error = ""
	_, failure := service.usersClient.SavePreferences(service.authorization, map[string]any{"categoryMappings": mappings})
	service.Saving = false
	FlowUtils{}.Either(failure == nil, service.saved, func() {
		service.Error = HTTPUtils{}.Message(failure, "Could not save folder roles.", "Network error.")
	})()
}

func (service *FolderRolesService) saved() {
	service.drafts = map[string]string{}
	service.reload()
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Folder roles updated."}, false)
}

// reload reads the folder catalog, shares it with the MCP section, and says
// whether the server refused the read.
func (service *FolderRolesService) reload() bool {
	catalog, failure := service.categoriesClient.Catalog(service.authorization)
	FlowUtils{}.When(failure == nil, func() { service.share(catalog) })()
	return HTTPUtils{}.Refused(failure)
}

func (service *FolderRolesService) share(catalog CategoryCatalog) {
	service.Accounts = append([]CategoryAccount{}, catalog.Accounts...)
	service.applicationEventPublisher.PublishEvent(CategoryCatalog{Accounts: service.Accounts}, true)
}

func (service *FolderRolesService) draftKey(domain string, role string) string {
	return domain + "\x00" + role
}
