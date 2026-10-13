// ExtensionsHelperTest calls every method of ExtensionsHelper on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(ExtensionsTestSetup)
type ExtensionsHelperTest struct {
	behaviors *BehaviorContext
}

func (test *ExtensionsHelperTest) TestPutNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&ExtensionsHelper{}).Put(map[string]any{"owner": "ada@example.com"})
		return err
	})
}

func (test *ExtensionsHelperTest) TestListForOwnerNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&ExtensionsHelper{}).ListForOwner("ada@example.com")
		return err
	})
}

func (test *ExtensionsHelperTest) TestFindForOwnerNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&ExtensionsHelper{}).FindForOwner("ada@example.com", "extension-1")
		return err
	})
}

func (test *ExtensionsHelperTest) TestUpdateNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&ExtensionsHelper{}).Update("doc-1", map[string]any{"enabled": false})
		return err
	})
}

func (test *ExtensionsHelperTest) TestDeleteNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&ExtensionsHelper{}).Delete("doc-1")
		return err
	})
}
