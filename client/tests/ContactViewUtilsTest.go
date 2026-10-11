// ContactViewUtilsTest covers what the contacts page computes for its list,
// sidebar and detail panel.
// @Test(ShellTestSetup, {"partition":"tests"})
type ContactViewUtilsTest struct {
	behaviors *BehaviorContext
}

func (test *ContactViewUtilsTest) ada() Contact {
	return Contact{
		ID:          "c1",
		Name:        "Ada Lovelace",
		GivenName:   "Ada",
		FamilyName:  "Lovelace",
		Nicknames:   []string{"Countess", "Enchantress"},
		Emails:      []ContactValue{{Type: "work", Value: "ada@example.com"}},
		Phones:      []ContactValue{{Value: "+44 20 1234"}},
		Addresses:   []ContactAddress{{City: "London", Country: "UK"}},
		Memberships: []string{"g1"},
		Source:      "carddav",
	}
}

func (test *ContactViewUtilsTest) TestRowsMarkTheSelectedAndCheckedContacts() {
	rows := ContactViewUtils{}.Rows([]Contact{test.ada(), {ID: "c2", Phones: []ContactValue{{Value: "555"}}}}, "c1", []string{"c2"})
	Assert.Equals("Ada Lovelace", rows[0].Heading)
	Assert.Equals("ada@example.com", rows[0].Subtitle)
	Assert.Equals("AL", rows[0].Initials)
	Assert.Equals("Synced address book", rows[0].SourceLabel)
	Assert.Equals("contact-row active", rows[0].ClassName)
	Assert.Equals("true", rows[0].Selected)
	Assert.Equals(false, rows[0].Checked)
	Assert.Equals("555", rows[1].Heading)
	Assert.Equals("Personal", rows[1].SourceLabel)
	Assert.Equals("contact-row", rows[1].ClassName)
	Assert.IsTrue(rows[1].Checked)
}

func (test *ContactViewUtilsTest) TestInitialsComeFromTheEmailWithoutAName() {
	rows := ContactViewUtils{}.Rows([]Contact{{ID: "c3", Emails: []ContactValue{{Value: "jane.doe@example.com"}}}}, "", nil)
	Assert.Equals("JD", rows[0].Initials)
	Assert.Equals("Select jane.doe@example.com", rows[0].SelectLabel)
}

func (test *ContactViewUtilsTest) TestLabelsCountContacts() {
	Assert.Equals("1 contact", ContactViewUtils{}.CountLabel(1))
	Assert.Equals("3 contacts", ContactViewUtils{}.CountLabel(3))
	Assert.Equals("", ContactViewUtils{}.CheckedLabel(0))
	Assert.Equals("2 selected", ContactViewUtils{}.CheckedLabel(2))
	Assert.Equals("Export all", ContactViewUtils{}.ExportLabel(0))
	Assert.Equals("Export selected (2)", ContactViewUtils{}.ExportLabel(2))
}

func (test *ContactViewUtilsTest) TestASourceListIsActiveOnlyWithoutAGroup() {
	filters := ContactViewUtils{}.Filters("mail", "", "4 contacts")
	Assert.Equals("4 contacts", filters[0].Count)
	Assert.Equals("contacts-filter", filters[0].ClassName)
	Assert.Equals("contacts-filter active", filters[2].ClassName)
	grouped := ContactViewUtils{}.Filters("mail", "g1", "4 contacts")
	Assert.Equals("contacts-filter", grouped[2].ClassName)
}

func (test *ContactViewUtilsTest) TestGroupRowsNameTheirActions() {
	rows := ContactViewUtils{}.GroupRows([]ContactGroup{{ID: "g1", Name: "Family"}, {ID: "g2", Name: "Work"}}, "g2")
	Assert.Equals("contacts-filter", rows[0].ClassName)
	Assert.Equals("contacts-filter active", rows[1].ClassName)
	Assert.Equals("Rename Family", rows[0].RenameLabel)
	Assert.Equals("Delete Family", rows[0].DeleteLabel)
	Assert.Equals("Delete the “Work” group? Contacts will be kept.", rows[1].DeleteQuestion)
}

func (test *ContactViewUtilsTest) TestDetailLinksAndGroups() {
	detail := ContactViewUtils{}.Detail(test.ada(), []ContactGroup{{ID: "g1", Name: "Family"}})
	Assert.Equals("Ada Lovelace", detail.FullName)
	Assert.Equals("mailto:ada@example.com", detail.Emails[0].Href)
	Assert.Equals("work", detail.Emails[0].Type)
	Assert.Equals("tel:+44 20 1234", detail.Phones[0].Href)
	Assert.Equals("phone", detail.Phones[0].Type)
	Assert.Equals("London, UK", detail.Addresses[0].Value)
	Assert.Equals([]string{"Family"}, detail.GroupNames)
	Assert.Equals("Countess, Enchantress", detail.Nicknames)
	Assert.IsTrue(detail.ShowProfile)
	Assert.IsTrue(detail.ShowNotes)
	Assert.IsTrue(detail.CanMessage)
	Assert.Equals("Delete Ada Lovelace?", detail.DeleteQuestion)
}

func (test *ContactViewUtilsTest) TestDetailOfANamelessContact() {
	detail := ContactViewUtils{}.Detail(Contact{ID: "c4", ReadOnly: true}, nil)
	Assert.Equals("?", detail.Initials)
	Assert.Equals(false, detail.CanMessage)
	Assert.Equals(false, detail.ShowProfile)
	Assert.IsTrue(detail.ReadOnly)
	Assert.Equals("Delete this contact?", detail.DeleteQuestion)
}
