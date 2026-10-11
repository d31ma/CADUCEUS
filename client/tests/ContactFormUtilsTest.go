// ContactFormUtilsTest covers the contact editor's form: its rows, its edits
// and the request body it becomes.
// @Test(ShellTestSetup, {"partition":"tests"})
type ContactFormUtilsTest struct {
	behaviors *BehaviorContext
}

func (test *ContactFormUtilsTest) TestABlankFormHasOneEmptyRowOfEachKind() {
	form := ContactFormUtils{}.Blank("new")
	Assert.Equals(1, len(form.Emails))
	Assert.Equals(1, len(form.Phones))
	Assert.Equals(1, len(form.Urls))
	Assert.Equals(1, len(form.Addresses))
	Assert.Equals(36, len(form.Emails[0].Key))
	Assert.Equals([]string{}, form.Memberships)
}

func (test *ContactFormUtilsTest) TestAContactBecomesAForm() {
	form := ContactFormUtils{}.FromContact(Contact{
		Name:        "Ada",
		Nicknames:   []string{"Countess", "Enchantress"},
		Emails:      []ContactValue{{Type: "work", Value: "ada@example.com"}, {Value: "a@example.org"}},
		Memberships: []string{"g1"},
	}, "c1")
	Assert.Equals("Countess, Enchantress", form.Nicknames)
	Assert.Equals(2, len(form.Emails))
	Assert.Equals("work", form.Emails[0].Type)
	Assert.Equals(1, len(form.Phones))
	Assert.Equals("", form.Phones[0].Value)
	Assert.Equals([]string{"g1"}, form.Memberships)
}

func (test *ContactFormUtilsTest) TestTheDraftKeepsOnlyFilledRows() {
	form := ContactFormUtils{}.Blank("new")
	form = ContactFormUtils{}.WithField(form, "name", "  Ada  ")
	form = ContactFormUtils{}.WithField(form, "nicknames", " Countess, ,Enchantress ")
	form = ContactFormUtils{}.WithValue(form, ValueEdit{Kind: "emails", Key: form.Emails[0].Key, Field: "value", Value: " ada@example.com "})
	form = ContactFormUtils{}.WithAddress(form, form.Addresses[0].Key, "type", "home")
	draft := ContactFormUtils{}.Draft(form)
	Assert.Equals("Ada", draft.Name)
	Assert.Equals([]string{"Countess", "Enchantress"}, draft.Nicknames)
	Assert.Equals([]DraftValue{{Value: "ada@example.com"}}, draft.Emails)
	Assert.Equals([]DraftValue{}, draft.Phones)
	Assert.Equals([]DraftAddress{}, draft.Addresses)
	Assert.Equals("", ContactFormUtils{}.Problem(draft))
}

func (test *ContactFormUtilsTest) TestAnUnreachableDraftIsAProblem() {
	draft := ContactFormUtils{}.Draft(ContactFormUtils{}.Blank("new"))
	Assert.Equals("Add a name, email address, or phone number.", ContactFormUtils{}.Problem(draft))
}

func (test *ContactFormUtilsTest) TestUnknownFieldsChangeNothing() {
	form := ContactFormUtils{}.Blank("new")
	Assert.Equals(form, ContactFormUtils{}.WithField(form, "unknown", "x"))
	Assert.Equals(form, ContactFormUtils{}.WithValue(form, ValueEdit{Kind: "faxes", Key: "k", Field: "value", Value: "x"}))
}

func (test *ContactFormUtilsTest) TestRowsAreAddedAndRemoved() {
	form := ContactFormUtils{}.WithAddedValue(ContactFormUtils{}.Blank("new"), "phones", "second")
	Assert.Equals(2, len(form.Phones))
	form = ContactFormUtils{}.WithoutValue(form, "phones", form.Phones[0].Key, "replacement")
	Assert.Equals(1, len(form.Phones))
	kept := form.Phones[0].Key
	form = ContactFormUtils{}.WithoutValue(form, "phones", kept, "replacement")
	Assert.Equals(1, len(form.Phones))
	Assert.IsTrue(form.Phones[0].Key != kept)
}

func (test *ContactFormUtilsTest) TestTheLastAddressEmptiesInsteadOfGoing() {
	form := ContactFormUtils{}.Blank("new")
	form = ContactFormUtils{}.WithAddress(form, form.Addresses[0].Key, "city", "London")
	form = ContactFormUtils{}.WithAddedAddress(form, "second")
	Assert.Equals(2, len(form.Addresses))
	form = ContactFormUtils{}.WithoutAddress(form, form.Addresses[1].Key, "replacement")
	Assert.Equals("London", form.Addresses[0].City)
	form = ContactFormUtils{}.WithoutAddress(form, form.Addresses[0].Key, "replacement")
	Assert.Equals(1, len(form.Addresses))
	Assert.Equals("", form.Addresses[0].City)
}

func (test *ContactFormUtilsTest) TestToggledAddsOnceAndRemoves() {
	Assert.Equals([]string{"g1", "g2"}, ContactFormUtils{}.Toggled([]string{"g1", "g2"}, "g2", true))
	Assert.Equals([]string{"g1", "g2"}, ContactFormUtils{}.Toggled([]string{"g1"}, "g2", true))
	Assert.Equals([]string{"g2"}, ContactFormUtils{}.Toggled([]string{"g1", "g2"}, "g1", false))
}
