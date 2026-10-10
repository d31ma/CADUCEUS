// ContactsHelperTest calls every method of ContactsHelper on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(ContactsTestSetup)
type ContactsHelperTest struct {
	behaviors *BehaviorContext
}

func (test *ContactsHelperTest) contact() map[string]any {
	return map[string]any{"id": "contact-1", "ownerEmail": "ada@example.com", "domain": "example.com", "emails": []any{}}
}

func (test *ContactsHelperTest) group() map[string]any {
	return map[string]any{"id": "group-1", "ownerEmail": "ada@example.com", "domain": "example.com"}
}

func (test *ContactsHelperTest) TestListNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&ContactsHelper{}).List("ada@example.com")
		return err
	})
}

func (test *ContactsHelperTest) TestFindByIdNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, _, err := (&ContactsHelper{}).FindById("contact-1", "ada@example.com")
		return err
	})
}

func (test *ContactsHelperTest) TestFindByEmailNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&ContactsHelper{}).FindByEmail("ada@example.com", "bob@example.com")
		return err
	})
}

func (test *ContactsHelperTest) TestPutNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&ContactsHelper{}).Put(test.contact())
		return err
	})
}

func (test *ContactsHelperTest) TestReplaceNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&ContactsHelper{}).Replace("doc-1", test.contact())
		return err
	})
}

func (test *ContactsHelperTest) TestDeleteNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&ContactsHelper{}).Delete("doc-1", "example.com")
		return err
	})
}

func (test *ContactsHelperTest) TestListGroupsNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&ContactsHelper{}).ListGroups("ada@example.com")
		return err
	})
}

func (test *ContactsHelperTest) TestFindGroupByIdNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, _, err := (&ContactsHelper{}).FindGroupById("group-1", "ada@example.com")
		return err
	})
}

func (test *ContactsHelperTest) TestPutGroupNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&ContactsHelper{}).PutGroup(test.group())
		return err
	})
}

func (test *ContactsHelperTest) TestReplaceGroupNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&ContactsHelper{}).ReplaceGroup("doc-1", test.group())
		return err
	})
}

func (test *ContactsHelperTest) TestDeleteGroupNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&ContactsHelper{}).DeleteGroup("doc-1", test.group())
		return err
	})
}
