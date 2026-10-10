// DomainsHelperTest calls every method of DomainsHelper on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(DomainsTestSetup)
type DomainsHelperTest struct {
	behaviors *BehaviorContext
}

func (test *DomainsHelperTest) TestListNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&DomainsHelper{}).List([]string{"example.com"})
		return err
	})
}

func (test *DomainsHelperTest) TestFindEntryNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, _, err := (&DomainsHelper{}).FindEntry("example.com")
		return err
	})
}

func (test *DomainsHelperTest) TestPutNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&DomainsHelper{}).Put(map[string]any{"domain": "example.com", "routes": []any{}})
		return err
	})
}

func (test *DomainsHelperTest) TestUpdateRoutesNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&DomainsHelper{}).UpdateRoutes("doc-1", []any{})
		return err
	})
}

func (test *DomainsHelperTest) TestDeleteNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&DomainsHelper{}).Delete("doc-1")
		return err
	})
}
