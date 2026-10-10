import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// CategoryPickerService is the account-scoped "Move to folder" and "Apply
// labels" dialog, matching Mailspring's account boundary: one picker serves
// messages of one mailbox domain.
//
// @Service
type CategoryPickerService struct {
	// @Needs
	categoriesClient *CategoriesClient
	// @Needs
	inboxClient               *InboxClient
	applicationEventPublisher ApplicationEventPublisher

	Open        bool
	Mode        string
	Domain      string
	DomainLabel string
	Search      string
	Creating    bool
	Error       string
	Title       string
	Placeholder string
	Items       []CategoryPickerRow
	CanCreate   bool
	CreateText  string

	messages      []PickerMessage
	accounts      []CategoryAccount
	mailboxNames  map[string]string
	authorization string
}

// @OnEvent({"replay":true})
func (picker *CategoryPickerService) SessionChanged(session SessionChanged) {
	picker.authorization = "Bearer " + session.Token
	picker.mailboxNames = session.MailboxNames
	picker.loadCatalog()
	picker.derive()
	picker.applicationEventPublisher.PublishEvent(ViewSettled{Source: "category-picker"}, false)
}

// @OnEvent
func (picker *CategoryPickerService) CategoryPickerRequested(requested CategoryPickerRequested) {
	domain := CategoryPickerUtils{}.SingleDomain(requested.Messages)
	capable := picker.capabilities(picker.accountFor(domain))[picker.capability(requested.Mode)]
	picker.openOrRefuse(picker.refusal(domain, requested.Mode, capable), requested, domain)
	picker.derive()
}

// @OnEvent
func (picker *CategoryPickerService) CategoryPickerDismissed(_ CategoryPickerDismissed) {
	picker.Close()
}

// Close closes the picker and gives the focus back.
func (picker *CategoryPickerService) Close() {
	mode := picker.Mode
	picker.Open = false
	picker.Mode = picker.move()
	picker.Domain = ""
	picker.Search = ""
	picker.Creating = false
	picker.Error = ""
	picker.messages = []PickerMessage{}
	picker.derive()
	picker.applicationEventPublisher.PublishEvent(CategoryPickerClosed{Mode: mode}, false)
}

func (CategoryPickerService) move() string { return "move" }

// SearchChanged narrows the folders or labels shown.
func (picker *CategoryPickerService) SearchChanged(value string) {
	picker.Search = value
	picker.Error = ""
	picker.derive()
}

// Select moves the messages to a folder, or applies or removes a label.
func (picker *CategoryPickerService) Select(path string) {
	picker.applyWhenGiven(path)
	picker.derive()
}

// Create makes the typed folder or label in the mailbox, then applies it.
func (picker *CategoryPickerService) Create() {
	path := CategoryPickerUtils{}.NormalizePath(picker.Search)
	created, failure := picker.createWhenAllowed(path)
	picker.afterCreate(created, failure)
	picker.derive()
}

func (picker *CategoryPickerService) loadCatalog() {
	catalog, failure := picker.categoriesClient.Catalog(picker.authorization)
	picker.accounts = picker.accountsUnless(catalog, failure)
	picker.applicationEventPublisher.PublishEvent(CategoryCatalogChanged{Accounts: picker.accounts}, true)
}

func (CategoryPickerService) accountsUnless(catalog CategoryCatalog, failure error) []CategoryAccount {
	if failure != nil || catalog.Accounts == nil {
		return []CategoryAccount{}
	}
	return catalog.Accounts
}

func (picker *CategoryPickerService) refusal(domain string, mode string, capable bool) string {
	switch {
	case domain == "":
		return "Choose messages from one mailbox to change folders or labels."
	case !capable:
		return fmt.Sprintf("This mailbox does not support %s.", picker.capability(mode))
	default:
		return ""
	}
}

func (picker *CategoryPickerService) openOrRefuse(problem string, requested CategoryPickerRequested, domain string) {
	if problem != "" {
		picker.applicationEventPublisher.PublishEvent(ToastRequested{Message: problem}, false)
		return
	}
	picker.Open = true
	picker.Mode = requested.Mode
	picker.Domain = domain
	picker.Search = ""
	picker.Creating = false
	picker.Error = ""
	picker.messages = requested.Messages
	picker.applicationEventPublisher.PublishEvent(CategoryPickerOpened{Mode: requested.Mode}, false)
}

func (picker *CategoryPickerService) applyWhenGiven(path string) {
	if path != "" {
		picker.apply(path)
	}
}

func (picker *CategoryPickerService) apply(path string) {
	label := picker.Mode == picker.label()
	patch := map[bool]BehaviorValue{true: CategoryPickerUtils{}.LabelPatch(picker.messages, path), false: map[string]any{"folder": path}}[label]
	message := map[bool]string{true: "Updated labels.", false: fmt.Sprintf("Moved to %s.", path)}[label]
	result, failure := picker.inboxClient.Act(picker.authorization, map[string]any{"ids": picker.ids(), "patch": patch})
	picker.announce(result, failure, message)
}

func (CategoryPickerService) label() string { return "label" }

func (picker *CategoryPickerService) announce(result MailActionResult, failure error, message string) {
	if failure != nil || result.ActionID == "" {
		picker.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Mail action failed."}, false)
		return
	}
	picker.applicationEventPublisher.PublishEvent(MailActionApplied{ActionID: result.ActionID, Message: message, ExpiresAt: result.ExpiresAt, Emails: result.Emails}, false)
	picker.applicationEventPublisher.PublishEvent(CategoryPickerApplied{Mode: picker.Mode}, false)
	picker.Close()
}

func (picker *CategoryPickerService) createWhenAllowed(path string) (CategoryCreated, error) {
	if !picker.CanCreate || path == "" {
		return CategoryCreated{}, nil
	}
	picker.Creating = true
	picker.Error = ""
	kind := map[bool]string{true: picker.label(), false: "folder"}[picker.Mode == picker.label()]
	return picker.categoriesClient.Create(picker.authorization, map[string]any{"domain": picker.Domain, "kind": kind, "path": path})
}

func (picker *CategoryPickerService) afterCreate(created CategoryCreated, failure error) {
	switch {
	case failure != nil:
		picker.Creating = false
		picker.Error = "Category creation failed."
	case created.Category.Path != "":
		picker.loadCatalog()
		picker.apply(created.Category.Path)
	}
}

func (picker *CategoryPickerService) derive() {
	label := picker.Mode == picker.label()
	kind := map[bool]string{true: picker.label(), false: "folder"}[label]
	account := picker.accountFor(picker.Domain)
	items := picker.shownItems(CategoryPickerUtils{}.Items(account, kind, picker.Search, picker.messages), label)
	createPath := CategoryPickerUtils{}.NormalizePath(picker.Search)
	picker.Title = map[bool]string{true: "Apply labels", false: "Move to folder"}[label]
	picker.Placeholder = map[bool]string{true: "Label as…", false: "Move to…"}[label]
	picker.DomainLabel = cmp.Or(picker.mailboxNames[strings.ToLower(picker.Domain)], strings.ToLower(picker.Domain))
	picker.Items = picker.rows(items, label)
	picker.CanCreate = picker.creatable(createPath, account, items)
	picker.CreateText = map[bool]string{true: "", false: fmt.Sprintf("Create %s “%s”", kind, createPath)}[createPath == ""]
}

func (picker *CategoryPickerService) shownItems(items []CategoryPickerItem, label bool) []CategoryPickerItem {
	if label {
		return items
	}
	return slices.DeleteFunc(items, func(item CategoryPickerItem) bool {
		return !slices.ContainsFunc(picker.messages, func(message PickerMessage) bool { return message.Folder != item.Path })
	})
}

func (picker *CategoryPickerService) rows(items []CategoryPickerItem, label bool) []CategoryPickerRow {
	rows := []CategoryPickerRow{}
	for _, item := range items {
		check := map[string]string{"all": "✓", "mixed": "−"}[item.SelectedState]
		rows = append(rows, CategoryPickerRow{
			Path: item.Path, DisplayName: item.DisplayName, ParentPath: item.ParentPath, Role: item.Role,
			Glyph:        map[bool]string{true: check, false: "📁"}[label],
			AriaSelected: map[bool]string{true: "true", false: "false"}[item.SelectedState == "all"],
			Style:        fmt.Sprintf("padding-inline-start:%dpx", picker.indentBase()+min(picker.deepest(), item.Depth)*picker.indentStep()),
		})
	}
	return rows
}

func (CategoryPickerService) indentBase() int { return 12 }

func (CategoryPickerService) indentStep() int { return 18 }

func (CategoryPickerService) deepest() int { return 6 }

func (picker *CategoryPickerService) creatable(path string, account CategoryAccount, items []CategoryPickerItem) bool {
	if path != "" && account.Capabilities.Create && !slices.ContainsFunc(items, func(item CategoryPickerItem) bool { return item.Path == path }) {
		return true
	}
	return false
}

func (picker *CategoryPickerService) accountFor(domain string) CategoryAccount {
	found := slices.IndexFunc(picker.accounts, func(account CategoryAccount) bool { return account.Domain == domain })
	if found < 0 {
		return CategoryAccount{}
	}
	return picker.accounts[found]
}

func (picker *CategoryPickerService) capabilities(account CategoryAccount) map[string]bool {
	return map[string]bool{picker.labelsCapability(): account.Capabilities.Labels, picker.foldersCapability(): account.Capabilities.Folders}
}

func (CategoryPickerService) labelsCapability() string { return "labels" }

func (CategoryPickerService) foldersCapability() string { return "folders" }

func (picker *CategoryPickerService) capability(mode string) string {
	return map[bool]string{true: picker.labelsCapability(), false: picker.foldersCapability()}[mode == picker.label()]
}

func (picker *CategoryPickerService) ids() []string {
	ids := []string{}
	for _, message := range picker.messages {
		ids = append(ids, message.ID)
	}
	return ids
}
