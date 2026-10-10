import (
	"encoding/json"
	"slices"
)

// CategoryPickerService runs the folder and label picker of the open
// message: it loads the account's categories, filters them, creates a new
// one and applies the move or label change as an undoable mail action.
//
// @Service
type CategoryPickerService struct {
	// @Needs
	categoriesClient *CategoriesClient
	// @Needs
	inboxClient               *InboxClient
	secureRandom              SecureRandom
	applicationEventPublisher ApplicationEventPublisher

	Open        bool
	Mode        string
	Search      string
	Creating    bool
	Error       string
	Title       string
	Placeholder string
	Domain      string
	Items       []CategoryPickerItem
	CanCreate   bool
	CreateText  string
	EmptyKind   string

	accounts []CategoryAccount
	emailId  string
	folder   string
	labels   []string
	token    string
}

func (CategoryPickerService) moveMode() string { return "move" }

// @OnStart
func (service *CategoryPickerService) Start() {
	service.accounts = []CategoryAccount{}
	service.labels = []string{}
	service.Mode = service.moveMode()
	service.refresh()
}

// @OnEvent({"replay":true})
func (service *CategoryPickerService) SessionChanged(changed SessionChanged) {
	service.token = changed.Token
}

// @OnEvent({"replay":true})
func (service *CategoryPickerService) MessageOpened(opened OpenMessageChanged) {
	service.emailId = opened.EmailId
	service.Domain = opened.Domain
	service.folder = opened.Folder
	service.labels = opened.Labels
	service.refresh()
}

func (service *CategoryPickerService) OpenPicker(mode string) {
	service.loadCatalog(service.account().Domain == "")
	service.openFor(mode, service.capability(mode, service.account().Capabilities), service.ready(mode))
	service.refresh()
}

func (service *CategoryPickerService) Close() {
	service.Open = false
	service.Search = ""
	service.Creating = false
	service.Error = ""
	service.refresh()
}

func (service *CategoryPickerService) Dismiss(className string) {
	service.dismiss(className == "detail-category-picker-backdrop")
}

func (service *CategoryPickerService) SearchInput(value string) {
	service.Search = value
	service.Error = ""
	service.refresh()
}

func (service *CategoryPickerService) Select(path string) {
	service.apply(path)
}

func (service *CategoryPickerService) Create() {
	service.create(CategoryPickerUtils{}.NormalizePickerPath(service.Search))
}

func (service *CategoryPickerService) authorization() string {
	return "Bearer " + service.token
}

func (CategoryPickerService) labelMode() string { return "label" }

func (service *CategoryPickerService) kind() string {
	return TextUtils{}.Choose(service.Mode == service.labelMode(), service.labelMode(), "folder")
}

func (service *CategoryPickerService) account() CategoryAccount {
	index := slices.IndexFunc(service.accounts, func(account CategoryAccount) bool { return account.Domain == service.Domain })
	return service.accountAt(index)
}

func (service *CategoryPickerService) accountAt(index int) CategoryAccount {
	if index < 0 {
		return CategoryAccount{Categories: []Category{}}
	}
	return service.accounts[index]
}

func (service *CategoryPickerService) loadCatalog(needed bool) {
	if needed && service.emailId != "" {
		service.fetchCatalog()
	}
}

func (service *CategoryPickerService) fetchCatalog() {
	catalog, failure := service.categoriesClient.Catalog(service.authorization())
	service.catalogLoaded(catalog, failure == nil)
}

func (service *CategoryPickerService) catalogLoaded(catalog CategoryCatalog, loaded bool) {
	if !loaded {
		service.toast("Could not load folders and labels.")
		return
	}
	service.accounts = catalog.Accounts
}

func (service *CategoryPickerService) capability(mode string, capabilities CategoryCapabilities) string {
	names := map[string]string{service.labelMode(): "labels", service.moveMode(): "folders"}
	supported := map[string]bool{service.labelMode(): capabilities.Labels, service.moveMode(): capabilities.Folders}
	return TextUtils{}.Choose(supported[mode], "", names[mode])
}

func (service *CategoryPickerService) ready(mode string) bool {
	known := slices.Contains([]string{service.labelMode(), service.moveMode()}, mode)
	if known && service.account().Domain != "" && service.emailId != "" {
		return true
	}
	return false
}

func (CategoryPickerService) period() string { return "." }

func (service *CategoryPickerService) openFor(mode string, missing string, ready bool) {
	switch {
	case !ready:
	case missing != "":
		service.toast("This mailbox does not support " + missing + service.period())
	default:
		service.show(mode)
	}
}

func (service *CategoryPickerService) show(mode string) {
	service.Mode = mode
	service.Search = ""
	service.Creating = false
	service.Error = ""
	service.Open = true
	service.applicationEventPublisher.PublishEvent(FocusRequested{Selector: "[data-detail-category-picker] .detail-category-picker-search"}, false)
}

func (service *CategoryPickerService) dismiss(onBackdrop bool) {
	if onBackdrop {
		service.Close()
	}
}

func (service *CategoryPickerService) refresh() {
	labelled := service.Mode == service.labelMode()
	service.Title = TextUtils{}.Choose(labelled, "Apply labels", "Move to folder")
	service.Placeholder = TextUtils{}.Choose(labelled, "Label as…", "Move to…")
	service.EmptyKind = TextUtils{}.Choose(labelled, "labels", "folders")
	service.Items = service.withoutCurrentFolder(CategoryPickerUtils{}.BuildItems(service.account(), service.kind(), service.Search, service.labels))
	path := CategoryPickerUtils{}.NormalizePickerPath(service.Search)
	service.CanCreate = service.canCreate(path)
	service.CreateText = TextUtils{}.Choose(path == "", "", "Create "+service.kind()+" “"+path+"”")
}

func (service *CategoryPickerService) withoutCurrentFolder(items []CategoryPickerItem) []CategoryPickerItem {
	if service.Mode == service.labelMode() {
		return items
	}
	return slices.DeleteFunc(items, func(item CategoryPickerItem) bool { return item.Path == service.folder })
}

func (service *CategoryPickerService) canCreate(path string) bool {
	exists := slices.ContainsFunc(service.Items, func(item CategoryPickerItem) bool { return item.Path == path })
	if path != "" && service.account().Capabilities.Create && !exists {
		return true
	}
	return false
}

func (service *CategoryPickerService) create(path string) {
	if service.CanCreate && path != "" && service.emailId != "" {
		service.createNow(path)
	}
}

func (service *CategoryPickerService) createNow(path string) {
	service.Creating = true
	service.Error = ""
	created, failure := service.categoriesClient.Create(map[string]any{"domain": service.Domain, "kind": service.kind(), "path": path}, service.authorization())
	service.created(created, failure)
}

func (service *CategoryPickerService) created(created CreatedCategory, failure error) {
	if failure != nil || created.Category.Path == "" {
		service.Creating = false
		service.Error = "Category creation failed."
		return
	}
	service.fetchCatalog()
	service.apply(created.Category.Path)
}

func (service *CategoryPickerService) apply(path string) {
	labelled := service.Mode == service.labelMode()
	patches := map[bool]map[string]any{true: CategoryPickerUtils{}.LabelPatch(service.labels, path), false: {"folder": path}}
	message := TextUtils{}.Choose(labelled, "Updated labels.", "Moved to "+path+service.period())
	requestId, _ := service.secureRandom.UUID()
	body := map[string]any{"ids": []string{service.emailId}, "patch": patches[labelled], "clientRequestId": requestId}
	result, failure := service.inboxClient.Act(body, service.authorization())
	encoded, _ := json.Marshal(body)
	service.applied(result, FailureUtils{}.Code(failure), OfflineRequestRequested{Id: requestId, Kind: "mail-action", Path: "/inbox/actions", Method: "POST", Body: string(encoded), Message: message, EmailId: service.emailId})
}

func (service *CategoryPickerService) applied(result MailActionResult, code string, offline OfflineRequestRequested) {
	switch code {
	case "":
		service.changed(result, offline.Message)
	case FailureUtils{}.OfflineCode():
		service.applicationEventPublisher.PublishEvent(offline, false)
	default:
		service.toast("Mail action failed.")
	}
}

func (service *CategoryPickerService) changed(result MailActionResult, message string) {
	for _, value := range result.Emails {
		service.publishChange(RecordUtils{}.Object(value))
	}
	service.applicationEventPublisher.PublishEvent(MailActionCompleted{ActionId: result.ActionId, ExpiresAt: result.ExpiresAt, Message: message}, false)
	service.applicationEventPublisher.PublishEvent(InboxRefreshRequested{Reason: "category"}, false)
	service.Close()
}

func (service *CategoryPickerService) publishChange(record map[string]any) {
	email := EmailRecordUtils{}.Email(record)
	service.applicationEventPublisher.PublishEvent(MessageChanged{EmailId: email.Id, Folder: email.Folder, SystemFolder: email.SystemFolder, Labels: email.Labels, Read: email.Read, Starred: email.Starred}, false)
}

func (service *CategoryPickerService) toast(message string) {
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}
