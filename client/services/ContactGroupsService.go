import "strings"

// ContactGroupsService is the contacts page's group dialog, which creates and
// renames groups, and the deletion of a group once it was confirmed. It
// announces each change in ContactGroupsChanged.
// @Service
type ContactGroupsService struct {
	// @Needs
	contactsClient            *ContactsClient
	applicationEventPublisher ApplicationEventPublisher
	Open                      bool
	Title                     string
	Name                      string
	Error                     string
	editingID                 string
	token                     string
}

// HearSession keeps the bearer token.
// @OnEvent({"replay":true})
func (groups *ContactGroupsService) HearSession(changed SessionChanged) {
	groups.token = changed.Token
}

// HearEscape closes the dialog.
// @OnEvent
func (groups *ContactGroupsService) HearEscape(pressed EscapePressed) {
	FlowUtils{}.When(groups.Open, groups.close)()
}

// HearBackdrop closes the dialog on a click beside it.
// @OnEvent
func (groups *ContactGroupsService) HearBackdrop(clicked BackdropClicked) {
	groups.closeFor(clicked.Dialog)
}

// HearSubmit saves the dialog when Enter submits its form.
// @OnEvent
func (groups *ContactGroupsService) HearSubmit(submitted FormSubmitted) {
	groups.saveFor(submitted.Form)
}

// HearConfirmation deletes the group whose deletion was confirmed.
// @OnEvent
func (groups *ContactGroupsService) HearConfirmation(given ConfirmationGiven) {
	groups.deleteFor(given)
}

// OpenNew opens the dialog for a new group.
func (groups *ContactGroupsService) OpenNew() {
	groups.show("", "")
}

// OpenRename opens the dialog to rename a group.
func (groups *ContactGroupsService) OpenRename(id string, name string) {
	groups.show(id, name)
}

// Close closes the dialog and returns focus to what opened it.
func (groups *ContactGroupsService) Close() {
	groups.close()
}

// SetName keeps the group name field.
func (groups *ContactGroupsService) SetName(value string) {
	groups.Name = value
}

// Save creates or renames the group.
func (groups *ContactGroupsService) Save() error {
	groups.save()
	return nil
}

func (groups *ContactGroupsService) closeFor(dialog string) {
	if dialog == groups.names().Dialog {
		groups.close()
	}
}

func (groups *ContactGroupsService) saveFor(form string) {
	if form == groups.names().Dialog {
		groups.save()
	}
}

func (groups *ContactGroupsService) deleteFor(given ConfirmationGiven) {
	if given.Action == groups.names().Delete {
		groups.delete(given.ID)
	}
}

func (groups *ContactGroupsService) show(id string, name string) {
	groups.editingID = id
	groups.Name = name
	groups.Error = ""
	groups.Open = true
	groups.Title = map[bool]string{true: "New group", false: "Rename group"}[id == ""]
	groups.applicationEventPublisher.PublishEvent(FocusRequested{Selector: "[data-contact-group-name]", Remember: true}, false)
}

func (groups *ContactGroupsService) close() {
	groups.Open = false
	groups.editingID = ""
	groups.Name = ""
	groups.Error = ""
	groups.applicationEventPublisher.PublishEvent(FocusRequested{Restore: true}, false)
}

func (groups *ContactGroupsService) bearer() string {
	return "Bearer " + groups.token
}

func (groups *ContactGroupsService) save() {
	name := strings.TrimSpace(groups.Name)
	FlowUtils{}.Either(name == "", groups.required, func() { groups.submit(name) })()
}

func (groups *ContactGroupsService) required() {
	groups.Error = "Group name is required."
}

func (groups *ContactGroupsService) submit(name string) {
	answer, failure := groups.request(map[string]string{"name": name})()
	groups.afterSave(answer, failure)
}

func (groups *ContactGroupsService) request(body map[string]string) func() (ContactGroup, error) {
	id := groups.editingID
	if id != "" {
		return func() (ContactGroup, error) { return groups.contactsClient.RenameGroup(groups.bearer(), id, body) }
	}
	return func() (ContactGroup, error) { return groups.contactsClient.CreateGroup(groups.bearer(), body) }
}

// afterSave keeps a refused save in the dialog with a fixed message, because
// the transport hands over no answer body.
func (groups *ContactGroupsService) afterSave(answer ContactGroup, failure error) {
	if (HTTPUtils{}).Missed(failure, answer.ID) {
		groups.Error = HTTPUtils{}.Message(failure, "Group could not be saved.", groups.offline())
		return
	}
	groups.close()
	groups.changed("", "Group saved.")
}

func (groups *ContactGroupsService) delete(id string) {
	_, failure := groups.contactsClient.DeleteGroup(groups.bearer(), id)
	FlowUtils{}.Either(failure == nil, func() { groups.changed(id, "Group deleted. Contacts were kept.") }, func() { groups.toast(HTTPUtils{}.Message(failure, "Group could not be deleted.", groups.offline())) })()
}

func (groups *ContactGroupsService) changed(deletedID string, message string) {
	groups.applicationEventPublisher.PublishEvent(ContactGroupsChanged{DeletedID: deletedID}, false)
	groups.toast(message)
}

func (groups *ContactGroupsService) toast(message string) {
	groups.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (ContactGroupsService) offline() string {
	return "The server could not be reached. Check your connection and try again."
}

func (ContactGroupsService) names() ContactGroupNames {
	return ContactGroupNames{Dialog: "contact-group", Delete: "contact-group-delete"}
}
