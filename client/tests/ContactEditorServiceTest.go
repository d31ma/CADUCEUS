// ContactEditorServiceTest covers the contact editor's steps that run before
// any request.
// @Test(ShellTestSetup, {"partition":"tests"})
type ContactEditorServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ContactEditorServiceTest) editor(selected Contact) *ContactEditorService {
	editor := &ContactEditorService{}
	editor.Start()
	editor.HearShown(ContactsShown{
		Selected:     selected,
		Groups:       []ContactGroup{{ID: "g1", Name: "Family"}},
		AddressBooks: []AddressBook{{AccountID: "ext_1", AddressBookID: "book", Name: "Contacts", AccountLabel: "Work"}},
	})
	return editor
}

func (test *ContactEditorServiceTest) TestANewContactOpensBlank() {
	editor := test.editor(Contact{})
	editor.OpenNew()
	Assert.IsTrue(editor.Open)
	Assert.Equals("New contact", editor.Title)
	Assert.Equals(" has-selection", editor.PageClass)
	Assert.Equals(1, len(editor.Form.Emails))
	Assert.IsTrue(editor.ShowDestinations)
	Assert.IsTrue(editor.ShowGroups)
	Assert.Equals("Work — Contacts", editor.Destinations[1].Label)
}

func (test *ContactEditorServiceTest) TestAnEmptyContactIsNotSaved() {
	editor := test.editor(Contact{})
	editor.OpenNew()
	editor.Save()
	Assert.Equals("Add a name, email address, or phone number.", editor.Error)
	Assert.IsTrue(editor.Open)
}

func (test *ContactEditorServiceTest) TestASyncedAddressBookHasNoGroups() {
	editor := test.editor(Contact{})
	editor.OpenNew()
	editor.ToggleGroup("g1", true)
	Assert.IsTrue(editor.GroupChoices[0].Checked)
	editor.SetDestination("ext_1|book")
	Assert.Equals("ext_1|book", editor.Destination)
	Assert.Equals([]string{}, editor.Form.Memberships)
	Assert.Equals(false, editor.ShowGroups)
	editor.SetDestination("missing")
	Assert.Equals("local", editor.Destination)
}

func (test *ContactEditorServiceTest) TestAReadOnlyContactIsNotEdited() {
	editor := test.editor(Contact{ID: "c1", Name: "Ada", ReadOnly: true})
	editor.Edit()
	Assert.Equals(false, editor.Open)
}

func (test *ContactEditorServiceTest) TestEditingFillsTheFormAndAnotherSelectionCloses() {
	editor := test.editor(Contact{ID: "c1", Name: "Ada", Memberships: []string{"g1"}})
	editor.Edit()
	Assert.IsTrue(editor.Open)
	Assert.Equals("Edit contact", editor.Title)
	Assert.Equals("Ada", editor.Form.Name)
	Assert.Equals(false, editor.ShowDestinations)
	editor.SetField("company", "Analytical Engines")
	Assert.Equals("Analytical Engines", editor.Form.Company)
	editor.HearShown(ContactsShown{Selected: Contact{ID: "c2"}})
	Assert.Equals(false, editor.Open)
}
