// UsersHelperTest calls every method of UsersHelper on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(UsersTestSetup)
type UsersHelperTest struct {
	behaviors *BehaviorContext
}

func (test *UsersHelperTest) user() map[string]any {
	return map[string]any{"email": "Ada@Example.com", "domains": []any{"example.com"}}
}

func (test *UsersHelperTest) TestListNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&UsersHelper{}).List()
		return err
	})
}

func (test *UsersHelperTest) TestFindByEmailNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, _, err := (&UsersHelper{}).FindByEmail("ada@example.com")
		return err
	})
}

func (test *UsersHelperTest) TestPutNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&UsersHelper{}).Put(test.user())
		return err
	})
}

func (test *UsersHelperTest) TestUpsertByEmailNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&UsersHelper{}).UpsertByEmail(test.user())
		return err
	})
}

func (test *UsersHelperTest) TestDeleteNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&UsersHelper{}).Delete("doc-1")
		return err
	})
}

func (test *UsersHelperTest) TestUpdateAliasesNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&UsersHelper{}).UpdateAliases("doc-1", []any{"ada.l@example.com"}, "ada@example.com")
		return err
	})
}

func (test *UsersHelperTest) TestUpdatePreferencesNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&UsersHelper{}).UpdatePreferences("doc-1", test.user())
		return err
	})
}

func (test *UsersHelperTest) TestUpdateMcpPreferencesNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&UsersHelper{}).UpdateMcpPreferences("doc-1", map[string]any{"enabled": false})
		return err
	})
}

func (test *UsersHelperTest) TestUpdateCategoryCatalogNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&UsersHelper{}).UpdateCategoryCatalog("doc-1", map[string]any{})
		return err
	})
}
