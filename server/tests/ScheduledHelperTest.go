// ScheduledHelperTest calls every method of ScheduledHelper on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(ScheduledTestSetup)
type ScheduledHelperTest struct {
	behaviors *BehaviorContext
}

func (test *ScheduledHelperTest) TestPutNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&ScheduledHelper{}).Put(map[string]any{"id": "send-1"})
		return err
	})
}

func (test *ScheduledHelperTest) TestListForOwnerNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&ScheduledHelper{}).ListForOwner("ada@example.com")
		return err
	})
}

func (test *ScheduledHelperTest) TestListAllNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&ScheduledHelper{}).ListAll()
		return err
	})
}

func (test *ScheduledHelperTest) TestFindByIdNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, _, err := (&ScheduledHelper{}).FindById("send-1")
		return err
	})
}

func (test *ScheduledHelperTest) TestDeleteNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&ScheduledHelper{}).Delete("doc-1")
		return err
	})
}

func (test *ScheduledHelperTest) TestUpdateNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&ScheduledHelper{}).Update("doc-1", map[string]any{"status": "sent"})
		return err
	})
}
