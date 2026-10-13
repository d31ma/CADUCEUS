// SendRequestsHelperTest calls every method of SendRequestsHelper on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(SendRequestsTestSetup)
type SendRequestsHelperTest struct {
	behaviors *BehaviorContext
}

func (test *SendRequestsHelperTest) TestPutNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&SendRequestsHelper{}).Put(map[string]any{"ownerEmail": "ada@example.com"})
		return err
	})
}

func (test *SendRequestsHelperTest) TestFindNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, _, err := (&SendRequestsHelper{}).Find("Ada@example.com", "key-1")
		return err
	})
}

func (test *SendRequestsHelperTest) TestUpdateNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&SendRequestsHelper{}).Update("doc-1", map[string]any{"status": "sent"})
		return err
	})
}
