import "slices"

// ContactEditorService is the contact editor: a new contact, saved here or to
// a synced CardDAV address book, or an edit of the selected contact. It hears
// the selection in ContactsShown and announces a save in ContactSaved; a
// CardDAV save is followed by ContactSyncService.
// @Service
type ContactEditorService struct {
	// @Needs
	contactsClient            *ContactsClient
	applicationEventPublisher ApplicationEventPublisher
	Open                      bool
	Title                     string
	Saving                    bool
	Error                     string
	Form                      ContactForm
	Destination               string
	Destinations              []DestinationOption
	ShowDestinations          bool
	GroupChoices              []GroupChoice
	ShowGroups                bool
	PageClass                 string
	token                     string
	contactID                 string
	shown                     ContactsShown
}

// Start shows the editor closed.
// @OnStart
func (editor *ContactEditorService) Start() {
	editor.Destination = editor.names().Local
	editor.refresh()
}

// HearSession keeps the bearer token.
// @OnEvent({"replay":true})
func (editor *ContactEditorService) HearSession(changed SessionChanged) {
	editor.token = changed.Token
}

// HearShown follows the groups and address books, and closes the editor when
// another contact is selected.
// @OnEvent({"replay":true})
func (editor *ContactEditorService) HearShown(shown ContactsShown) {
	editor.shown = shown
	FlowUtils{}.When(shown.Selected.ID != editor.contactID, editor.close)()
	editor.refresh()
}

// OpenNew starts a new contact.
func (editor *ContactEditorService) OpenNew() {
	editor.contactID = ""
	editor.Destination = editor.names().Local
	editor.begin(ContactFormUtils{}.Blank("new contact"), true)
}

// Edit starts editing the selected contact, unless it is read-only.
func (editor *ContactEditorService) Edit() {
	selected := editor.shown.Selected
	FlowUtils{}.When(!slices.Contains([]bool{selected.ID != "", !selected.ReadOnly}, false), editor.editSelected)()
}

// Cancel closes the editor without saving.
func (editor *ContactEditorService) Cancel() {
	editor.close()
	editor.refresh()
}

// Save creates or updates the contact.
func (editor *ContactEditorService) Save() error {
	draft := ContactFormUtils{}.Draft(editor.Form)
	book, found := editor.remoteBook()
	problem := editor.problem(draft, found)
	FlowUtils{}.Either(problem == "", func() { editor.submit(editor.prepared(draft, book, found)) }, func() { editor.Error = problem })()
	editor.refresh()
	return nil
}

// SetField keeps one text field, such as "givenName".
func (editor *ContactEditorService) SetField(field string, value string) {
	editor.Form = ContactFormUtils{}.WithField(editor.Form, field, value)
}

// SetValueType keeps the type of one email, phone or website row.
func (editor *ContactEditorService) SetValueType(kind string, key string, value string) {
	editor.Form = ContactFormUtils{}.WithValue(editor.Form, ValueEdit{Kind: kind, Key: key, Field: "type", Value: value})
}

// SetValueText keeps the text of one email, phone or website row.
func (editor *ContactEditorService) SetValueText(kind string, key string, value string) {
	editor.Form = ContactFormUtils{}.WithValue(editor.Form, ValueEdit{Kind: kind, Key: key, Field: "value", Value: value})
}

// AddValue adds an empty email, phone or website row.
func (editor *ContactEditorService) AddValue(kind string) {
	editor.Form = ContactFormUtils{}.WithAddedValue(editor.Form, kind, kind)
}

// RemoveValue removes one email, phone or website row.
func (editor *ContactEditorService) RemoveValue(kind string, key string) {
	editor.Form = ContactFormUtils{}.WithoutValue(editor.Form, kind, key, kind+key)
}

// SetAddress keeps one part of one address.
func (editor *ContactEditorService) SetAddress(key string, field string, value string) {
	editor.Form = ContactFormUtils{}.WithAddress(editor.Form, key, field, value)
}

// AddAddress adds an empty address.
func (editor *ContactEditorService) AddAddress() {
	editor.Form = ContactFormUtils{}.WithAddedAddress(editor.Form, editor.contactID)
}

// RemoveAddress removes one address.
func (editor *ContactEditorService) RemoveAddress(key string) {
	editor.Form = ContactFormUtils{}.WithoutAddress(editor.Form, key, key)
}

// ToggleGroup adds the contact to a group or takes it out.
func (editor *ContactEditorService) ToggleGroup(id string, checked bool) {
	editor.Form.Memberships = ContactFormUtils{}.Toggled(editor.Form.Memberships, id, checked)
	editor.refresh()
}

// SetDestination chooses where a new contact is saved. A CardDAV address
// book has no groups, so choosing one clears them.
func (editor *ContactEditorService) SetDestination(value string) {
	editor.Destination = editor.destinationOr(value)
	editor.Form.Memberships = editor.groupsFor(editor.Form.Memberships)
	editor.refresh()
}

func (editor *ContactEditorService) editSelected() {
	editor.contactID = editor.shown.Selected.ID
	editor.begin(ContactFormUtils{}.FromContact(editor.shown.Selected, editor.contactID), false)
}

func (editor *ContactEditorService) begin(form ContactForm, fresh bool) {
	editor.Form = form
	editor.Error = ""
	editor.Saving = false
	editor.Open = true
	editor.refresh()
	editor.applicationEventPublisher.PublishEvent(ContactEditorOpened{New: fresh}, false)
	editor.applicationEventPublisher.PublishEvent(FocusRequested{Selector: "[data-contact-name]"}, false)
}

func (editor *ContactEditorService) close() {
	editor.Open = false
	editor.Saving = false
	editor.Error = ""
	editor.contactID = ""
}

func (editor *ContactEditorService) bearer() string {
	return "Bearer " + editor.token
}

// remoteBook is the CardDAV address book a new contact goes to, when one was
// chosen and still exists.
func (editor *ContactEditorService) remoteBook() (AddressBook, bool) {
	index := slices.IndexFunc(editor.shown.AddressBooks, func(book AddressBook) bool { return editor.destinationValue(book) == editor.Destination })
	if index < 0 {
		return AddressBook{}, false
	}
	return editor.shown.AddressBooks[index], true
}

func (editor *ContactEditorService) problem(draft ContactDraft, found bool) string {
	if !slices.Contains([]bool{editor.contactID == "", editor.Destination != editor.names().Local, !found}, false) {
		return "Choose an available address book."
	}
	return ContactFormUtils{}.Problem(draft)
}

// prepared adds what a CardDAV write needs: the address book of a new contact
// and an idempotency key.
func (editor *ContactEditorService) prepared(draft ContactDraft, book AddressBook, found bool) ContactDraft {
	return editor.withUpdateKey(editor.withBook(draft, book, found))
}

func (editor *ContactEditorService) withBook(draft ContactDraft, book AddressBook, found bool) ContactDraft {
	if !slices.Contains([]bool{editor.contactID == "", found}, false) {
		draft.Source = editor.names().Synced
		draft.AccountID = book.AccountID
		draft.AddressBookID = book.AddressBookID
		draft.IdempotencyKey = "contact-create-" + IdentityUtils{}.UUID(book.AddressBookID+draft.Name)
		draft.Memberships = []string{}
	}
	return draft
}

func (editor *ContactEditorService) withUpdateKey(draft ContactDraft) ContactDraft {
	if editor.shown.Selected.Source == editor.names().Synced {
		draft.IdempotencyKey = "contact-update-" + IdentityUtils{}.UUID(editor.contactID)
	}
	return draft
}

func (editor *ContactEditorService) submit(draft ContactDraft) {
	editor.Saving = true
	editor.Error = ""
	answer, failure := editor.request(draft)()
	editor.afterSave(answer, failure, draft.Source == editor.names().Synced)
}

func (editor *ContactEditorService) request(draft ContactDraft) func() (Contact, error) {
	id := editor.contactID
	if id != "" {
		return func() (Contact, error) { return editor.contactsClient.Update(editor.bearer(), id, draft) }
	}
	return func() (Contact, error) { return editor.contactsClient.Create(editor.bearer(), draft) }
}

// afterSave keeps a refused save in the editor with a fixed message, because
// the transport hands over no answer body.
func (editor *ContactEditorService) afterSave(answer Contact, failure error, created bool) {
	editor.Saving = false
	if (HTTPUtils{}).Missed(failure, answer.ID) {
		editor.Error = HTTPUtils{}.Message(failure, "Contact could not be saved.", "Network error while saving the contact.")
		return
	}
	editor.close()
	editor.announce(answer, created)
}

func (editor *ContactEditorService) announce(answer Contact, created bool) {
	pending := !slices.Contains([]string{answer.SyncState, answer.MutationID, answer.AccountID}, "")
	editor.applicationEventPublisher.PublishEvent(ContactSaved{ID: answer.ID, Pending: pending}, false)
	FlowUtils{}.When(pending, func() { editor.follow(answer, created) })()
}

func (editor *ContactEditorService) follow(answer Contact, created bool) {
	conflicts := map[bool]string{
		true:  "The CardDAV server already has a different contact at the new resource location.",
		false: "This contact changed on the CardDAV server. The server version was kept.",
	}
	editor.applicationEventPublisher.PublishEvent(ContactMutationStarted{
		ContactID:  answer.ID,
		MutationID: answer.MutationID,
		AccountID:  answer.AccountID,
		Kind:       "save",
		Conflict:   conflicts[created],
	}, false)
}

// destinationOr keeps a known address book, and the server's own contacts
// otherwise.
func (editor *ContactEditorService) destinationOr(value string) string {
	known := slices.ContainsFunc(editor.shown.AddressBooks, func(book AddressBook) bool { return editor.destinationValue(book) == value })
	if known {
		return value
	}
	return editor.names().Local
}

func (editor *ContactEditorService) groupsFor(ids []string) []string {
	if editor.Destination == editor.names().Local {
		return ids
	}
	return []string{}
}

func (editor *ContactEditorService) destinationValue(book AddressBook) string {
	return book.AccountID + editor.names().Separator + book.AddressBookID
}

// The view state.

func (editor *ContactEditorService) refresh() {
	editor.Title = map[bool]string{true: "New contact", false: "Edit contact"}[editor.contactID == ""]
	editor.Destinations = editor.destinations()
	editor.ShowDestinations = !slices.Contains([]bool{editor.contactID == "", len(editor.shown.AddressBooks) > 0}, false)
	editor.GroupChoices = editor.groupChoices()
	editor.ShowGroups = !slices.Contains([]bool{len(editor.shown.Groups) > 0, editor.canAssignGroups()}, false)
	editor.PageClass = map[bool]string{true: " has-selection"}[editor.Open]
}

// canAssignGroups is false for a CardDAV contact, whose address book has no
// groups.
func (editor *ContactEditorService) canAssignGroups() bool {
	if editor.contactID != "" {
		return editor.shown.Selected.Source != editor.names().Synced
	}
	return editor.Destination == editor.names().Local
}

func (editor *ContactEditorService) destinations() []DestinationOption {
	options := []DestinationOption{{Value: editor.names().Local, Label: "Personal contacts on this server", Selected: editor.Destination == editor.names().Local}}
	for _, book := range editor.shown.AddressBooks {
		options = append(options, editor.destination(book))
	}
	return options
}

func (editor *ContactEditorService) destination(book AddressBook) DestinationOption {
	account := TextUtils{}.FirstOf(TextUtils{}.FirstOf(book.AccountLabel, book.MailboxAddress), "External account")
	value := editor.destinationValue(book)
	return DestinationOption{Value: value, Label: account + " — " + TextUtils{}.FirstOf(book.Name, "Contacts"), Selected: value == editor.Destination}
}

func (editor *ContactEditorService) groupChoices() []GroupChoice {
	choices := []GroupChoice{}
	for _, group := range editor.shown.Groups {
		choices = append(choices, GroupChoice{ID: group.ID, Name: group.Name, Checked: slices.Contains(editor.Form.Memberships, group.ID)})
	}
	return choices
}

func (ContactEditorService) names() ContactEditorNames {
	return ContactEditorNames{Local: "local", Synced: "carddav", Separator: "|"}
}
