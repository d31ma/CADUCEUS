// McpAccessTokensHelperTest calls every method of McpAccessTokensHelper on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(McpAccessTokensTestSetup)
type McpAccessTokensHelperTest struct {
	behaviors *BehaviorContext
}

func (test *McpAccessTokensHelperTest) TestPutNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&McpAccessTokensHelper{}).Put(map[string]any{"ownerEmail": "ada@example.com"})
		return err
	})
}

func (test *McpAccessTokensHelperTest) TestListForOwnerNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&McpAccessTokensHelper{}).ListForOwner("ada@example.com")
		return err
	})
}

func (test *McpAccessTokensHelperTest) TestFindByHashNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&McpAccessTokensHelper{}).FindByHash("hash-1")
		return err
	})
}

func (test *McpAccessTokensHelperTest) TestUpdateNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&McpAccessTokensHelper{}).Update("doc-1", map[string]any{"revokedAt": "2026-01-01T00:00:00.000Z"})
		return err
	})
}

func (test *McpAccessTokensHelperTest) TestDeleteNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&McpAccessTokensHelper{}).Delete("doc-1")
		return err
	})
}
