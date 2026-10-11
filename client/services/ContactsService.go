import (
	"encoding/json"
	"slices"
)

// ContactsService is the contacts page's list and detail panel: the search,
// the source and group filters, the checked contacts, the selected contact,
// deleting it and writing to it. The editor, the group dialog, the vCard
// import and export and the CardDAV change tracking are their own services;
// they hear the retained ContactsShown event and answer in events.
// @Service
type ContactsService struct {
	// @Needs
	contactsClient            *ContactsClient
	applicationEventPublisher ApplicationEventPublisher
	Contacts                  []ContactRow
	Filters                   []ContactFilter
	Groups                    []ContactGroupRow
	CountLabel                string
	CheckedLabel              string
	ExportLabel               string
	Loading                   bool
	Empty                     bool
	Error                     string
	CanReviewConflict         bool
	DismissLabel              string
	PageClass                 string
	HasSelection              bool
	Detail                    ContactDetail
	// @SessionState
	Query string
	// @SessionState
	SourceFilter string
	// @SessionState
	GroupFilter string
	// @SessionState
	SelectedID string
	// @SessionState
	Checked      []string
	conflict     bool
	contacts     []Contact
	groups       []ContactGroup
	addressBooks []AddressBook
	token        string
	opened       bool
}

// Start shows the loading state until the session arrives.
// @OnStart
func (contacts *ContactsService) Start() {
	contacts.Loading = true
	contacts.Checked = append([]string{}, contacts.Checked...)
	contacts.refresh()
}

// HearSession keeps the bearer token and, the first time someone is signed
// in, loads the groups, the address books and the contacts.
// @OnEvent({"replay":true})
func (contacts *ContactsService) HearSession(changed SessionChanged) {
	contacts.token = changed.Token
	FlowUtils{}.When(!slices.Contains([]bool{changed.SignedIn, !contacts.opened}, false), contacts.open)()
}

// HearTimer runs the search once typing pauses.
// @OnEvent
func (contacts *ContactsService) HearTimer(elapsed TimerElapsed) {
	FlowUtils{}.When(elapsed.Name == contacts.names().SearchTimer, contacts.reload)()
}

// HearConfirmation deletes the contact whose deletion was confirmed.
// @OnEvent
func (contacts *ContactsService) HearConfirmation(given ConfirmationGiven) {
	contacts.deleteWhenConfirmed(given.Action, given.ID)
}

// HearEditorOpened clears the alert, and the selection for a new contact.
// @OnEvent
func (contacts *ContactsService) HearEditorOpened(opened ContactEditorOpened) {
	contacts.dismiss()
	FlowUtils{}.When(opened.New, contacts.deselect)()
	contacts.show()
}

// HearSaved selects the saved contact; a CardDAV change shows its outcome
// once ContactSyncService hears that it settled.
// @OnEvent
func (contacts *ContactsService) HearSaved(saved ContactSaved) {
	contacts.SelectedID = saved.ID
	contacts.dismiss()
	FlowUtils{}.When(!saved.Pending, contacts.savedNow)()
	contacts.show()
}

// HearSettled shows how a CardDAV save or delete ended.
// @OnEvent
func (contacts *ContactsService) HearSettled(settled ContactMutationSettled) {
	failed := slices.Contains([]string{contacts.names().Conflict, "failed"}, settled.State)
	contacts.settledStep(settled, failed)()
	contacts.show()
}

// HearGroupsChanged reads the groups and the contacts again; a deleted group
// stops filtering the list.
// @OnEvent
func (contacts *ContactsService) HearGroupsChanged(changed ContactGroupsChanged) {
	FlowUtils{}.When(!slices.Contains([]bool{changed.DeletedID != "", contacts.GroupFilter == changed.DeletedID}, false), contacts.clearGroupFilter)()
	contacts.loadGroups()
	contacts.reload()
}

// HearImported reads the contacts again after a vCard import.
// @OnEvent
func (contacts *ContactsService) HearImported(imported ContactsImported) {
	contacts.reload()
}

// Search keeps the query and searches once typing pauses for 180 ms.
func (contacts *ContactsService) Search(value string) {
	contacts.Query = value
	contacts.applicationEventPublisher.PublishEvent(TimerRequested{Name: contacts.names().SearchTimer, DelayMs: 180}, false)
}

// SetSource lists the contacts of one source, or every contact for "".
func (contacts *ContactsService) SetSource(source string) error {
	contacts.SourceFilter = source
	contacts.GroupFilter = ""
	contacts.SelectedID = ""
	contacts.reload()
	return nil
}

// SetGroup lists the contacts of one group.
func (contacts *ContactsService) SetGroup(id string) error {
	contacts.GroupFilter = id
	contacts.SourceFilter = ""
	contacts.SelectedID = ""
	contacts.reload()
	return nil
}

// ToggleChecked checks or unchecks one contact for export.
func (contacts *ContactsService) ToggleChecked(id string, checked bool) {
	contacts.Checked = ContactFormUtils{}.Toggled(contacts.Checked, id, checked)
	contacts.show()
}

// Select shows one contact's details.
func (contacts *ContactsService) Select(id string) {
	contacts.SelectedID = id
	contacts.dismiss()
	contacts.show()
}

// CloseDetail returns to the list on a phone-sized screen.
func (contacts *ContactsService) CloseDetail() {
	contacts.deselect()
	contacts.dismiss()
	contacts.show()
}

// DismissError hides the alert; after a conflict it keeps the server version.
func (contacts *ContactsService) DismissError() {
	contacts.dismiss()
	contacts.show()
}

// Compose opens the composer addressed to the selected contact.
func (contacts *ContactsService) Compose() {
	prefill, _ := json.Marshal(map[string]string{"to": ContactViewUtils{}.PrimaryEmail(contacts.selected())})
	contacts.applicationEventPublisher.PublishEvent(StorageWritten{Area: "session", Values: map[string]string{"caduceus_compose_prefill": string(prefill)}, Removed: []string{}}, false)
	contacts.applicationEventPublisher.PublishEvent(NavigationRequested{Target: "/compose"}, false)
}

func (contacts *ContactsService) bearer() string {
	return "Bearer " + contacts.token
}

func (contacts *ContactsService) open() {
	contacts.opened = true
	contacts.loadGroups()
	contacts.loadAddressBooks()
	contacts.reload()
}

// reload reads the contacts and ends with ContactsShown, so the page renders
// what an event listener changed.
func (contacts *ContactsService) reload() {
	found, failure := contacts.contactsClient.List(contacts.bearer(), contacts.Query, contacts.SourceFilter, contacts.GroupFilter)
	contacts.contacts = append([]Contact{}, found...)
	contacts.Loading = false
	contacts.failedRead(failure)
	contacts.keepListed()
	contacts.show()
}

// failedRead shows why the list could not be read; a refused read asks
// SessionService whether the token still works, because the transport
// reports every error status alike.
func (contacts *ContactsService) failedRead(failure error) {
	if failure != nil {
		contacts.Error = HTTPUtils{}.Message(failure, "Contacts could not be loaded.", contacts.names().Offline)
		FlowUtils{}.When(HTTPUtils{}.Refused(failure), func() {
			contacts.applicationEventPublisher.PublishEvent(SessionCheckRequested{Reason: contacts.names().Session}, false)
		})()
	}
}

// loadGroups and loadAddressBooks show none when the read fails: a failed
// read answers no list.
func (contacts *ContactsService) loadGroups() {
	groups, _ := contacts.contactsClient.Groups(contacts.bearer())
	contacts.groups = append([]ContactGroup{}, groups...)
}

func (contacts *ContactsService) loadAddressBooks() {
	books, _ := contacts.contactsClient.AddressBooks(contacts.bearer())
	contacts.addressBooks = append([]AddressBook{}, books...)
}

// keepListed forgets checked and selected contacts that the list no longer
// holds.
func (contacts *ContactsService) keepListed() {
	listed := contacts.shownIdentifiers()
	contacts.Checked = slices.DeleteFunc(append([]string{}, contacts.Checked...), func(id string) bool { return !slices.Contains(listed, id) })
	contacts.SelectedID = map[bool]string{true: contacts.SelectedID}[slices.Contains(listed, contacts.SelectedID)]
}

func (contacts *ContactsService) shownIdentifiers() []string {
	ids := []string{}
	for _, contact := range contacts.contacts {
		ids = append(ids, contact.ID)
	}
	return ids
}

func (contacts *ContactsService) selected() Contact {
	index := slices.IndexFunc(contacts.contacts, func(contact Contact) bool { return contact.ID == contacts.SelectedID })
	if index < 0 {
		return Contact{}
	}
	return contacts.contacts[index]
}

func (contacts *ContactsService) deselect() {
	contacts.SelectedID = ""
}

func (contacts *ContactsService) dismiss() {
	contacts.Error = ""
	contacts.conflict = false
}

func (contacts *ContactsService) clearGroupFilter() {
	contacts.GroupFilter = ""
}

func (contacts *ContactsService) savedNow() {
	contacts.reload()
	contacts.toast("Contact saved.")
}

func (contacts *ContactsService) toast(message string) {
	contacts.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

// Deleting.

func (contacts *ContactsService) deleteWhenConfirmed(action string, id string) {
	if action == contacts.names().DeleteContact {
		contacts.delete(id)
	}
}

func (contacts *ContactsService) delete(id string) {
	answer, failure := contacts.contactsClient.Delete(contacts.bearer(), id, "contact-delete-"+IdentityUtils{}.UUID(id))
	contacts.deletedStep(id, answer, failure)()
	contacts.show()
}

func (contacts *ContactsService) deletedStep(id string, answer ContactDeletion, failure error) func() {
	switch {
	case failure != nil:
		return func() {
			contacts.toast(HTTPUtils{}.Message(failure, "Contact could not be deleted.", contacts.names().Offline))
		}
	case answer.Pending:
		return func() { contacts.followDelete(id, answer) }
	default:
		return func() { contacts.deleted(id) }
	}
}

func (contacts *ContactsService) followDelete(id string, answer ContactDeletion) {
	contacts.applicationEventPublisher.PublishEvent(ContactMutationStarted{
		ContactID:  id,
		MutationID: answer.MutationID,
		AccountID:  answer.AccountID,
		Kind:       contacts.names().Delete,
		Conflict:   "This contact changed on the CardDAV server and was not deleted. The server version was kept.",
	}, false)
}

func (contacts *ContactsService) deleted(id string) {
	contacts.Checked = ContactFormUtils{}.Toggled(contacts.Checked, id, false)
	contacts.deselect()
	contacts.dismiss()
	contacts.reload()
	contacts.toast("Contact deleted.")
}

// settledStep is what follows a settled CardDAV change: a failure reloads and
// shows the alert, a save reloads and confirms, and a delete forgets it.
func (contacts *ContactsService) settledStep(settled ContactMutationSettled, failed bool) func() {
	switch {
	case failed:
		return func() { contacts.reload(); contacts.showFailure(settled) }
	case settled.Kind == contacts.names().Delete:
		return func() { contacts.deleted(settled.ContactID) }
	default:
		return contacts.savedNow
	}
}

func (contacts *ContactsService) showFailure(settled ContactMutationSettled) {
	contacts.conflict = settled.State == contacts.names().Conflict
	contacts.Error = map[bool]string{true: settled.Conflict, false: "The CardDAV server rejected this contact change."}[contacts.conflict]
}

// The view state.

// show recomputes what the page shows and tells the other contact services.
func (contacts *ContactsService) show() {
	contacts.refresh()
	contacts.applicationEventPublisher.PublishEvent(ContactsShown{
		Selected:     contacts.selected(),
		Groups:       contacts.groups,
		AddressBooks: contacts.addressBooks,
		Checked:      contacts.Checked,
		GroupFilter:  contacts.GroupFilter,
	}, true)
}

func (contacts *ContactsService) refresh() {
	selected := contacts.selected()
	contacts.CountLabel = ContactViewUtils{}.CountLabel(len(contacts.contacts))
	contacts.Contacts = ContactViewUtils{}.Rows(contacts.contacts, contacts.SelectedID, contacts.Checked)
	contacts.Filters = ContactViewUtils{}.Filters(contacts.SourceFilter, contacts.GroupFilter, contacts.CountLabel)
	contacts.Groups = ContactViewUtils{}.GroupRows(contacts.groups, contacts.GroupFilter)
	contacts.CheckedLabel = ContactViewUtils{}.CheckedLabel(len(contacts.Checked))
	contacts.ExportLabel = ContactViewUtils{}.ExportLabel(len(contacts.Checked))
	contacts.Empty = !slices.Contains([]bool{contacts.Loading, len(contacts.contacts) > 0}, true)
	contacts.HasSelection = selected.ID != ""
	contacts.Detail = ContactViewUtils{}.Detail(selected, contacts.groups)
	contacts.PageClass = "contacts-page" + map[bool]string{true: " has-selection"}[contacts.HasSelection]
	contacts.CanReviewConflict = !slices.Contains([]bool{contacts.conflict, contacts.HasSelection, !selected.ReadOnly}, false)
	contacts.DismissLabel = map[bool]string{true: "Keep server version", false: "Dismiss"}[contacts.CanReviewConflict]
}

func (ContactsService) names() ContactsNames {
	return ContactsNames{
		SearchTimer:   "contacts-search",
		DeleteContact: "contact-delete",
		Save:          "save",
		Delete:        "delete",
		Conflict:      "conflict",
		Offline:       "The server could not be reached. Check your connection and try again.",
		Session:       "contacts",
	}
}
