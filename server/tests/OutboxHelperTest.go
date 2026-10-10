// OutboxHelperTest calls every method of OutboxHelper on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(OutboxTestSetup)
type OutboxHelperTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxHelperTest) TestPutNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&OutboxHelper{}).Put(map[string]any{"id": "undo-1"})
		return err
	})
}

func (test *OutboxHelperTest) TestFindByIdNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, _, err := (&OutboxHelper{}).FindById("undo-1")
		return err
	})
}

func (test *OutboxHelperTest) TestDeleteNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&OutboxHelper{}).Delete("doc-1")
		return err
	})
}

func (test *OutboxHelperTest) TestFindExpiredNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&OutboxHelper{}).FindExpired()
		return err
	})
}

func (test *OutboxHelperTest) TestFindDueNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&OutboxHelper{}).FindDue(EpochTime{}.Now())
		return err
	})
}

func (test *OutboxHelperTest) TestListAllNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&OutboxHelper{}).ListAll()
		return err
	})
}

func (test *OutboxHelperTest) TestUpdateNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&OutboxHelper{}).Update("doc-1", map[string]any{"attempts": 1})
		return err
	})
}
