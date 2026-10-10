// McpAuditEventsHelperTest calls every method of McpAuditEventsHelper on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(McpAuditEventsTestSetup)
type McpAuditEventsHelperTest struct {
	behaviors *BehaviorContext
}

func (test *McpAuditEventsHelperTest) TestPutNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&McpAuditEventsHelper{}).Put(map[string]any{"ownerEmail": "ada@example.com"})
		return err
	})
}

func (test *McpAuditEventsHelperTest) TestListForOwnerNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&McpAuditEventsHelper{}).ListForOwner("ada@example.com", 50)
		return err
	})
}

func (test *McpAuditEventsHelperTest) TestClearForOwnerNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&McpAuditEventsHelper{}).ClearForOwner("ada@example.com")
		return err
	})
}
