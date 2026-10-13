// FollowUpRemindersHelperTest calls every method of FollowUpRemindersHelper on an instance that Tachyon did
// not build, so it has no request database: each method fails as soon as it
// reaches FYLO. The route suites check the stored documents over HTTP.
// @Test(FollowUpRemindersTestSetup)
type FollowUpRemindersHelperTest struct {
	behaviors *BehaviorContext
}

func (test *FollowUpRemindersHelperTest) TestPutNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&FollowUpRemindersHelper{}).Put(map[string]any{"id": "reminder-1"})
		return err
	})
}

func (test *FollowUpRemindersHelperTest) TestListForOwnerNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&FollowUpRemindersHelper{}).ListForOwner("ada@example.com")
		return err
	})
}

func (test *FollowUpRemindersHelperTest) TestListPendingNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&FollowUpRemindersHelper{}).ListPending("ada@example.com")
		return err
	})
}

func (test *FollowUpRemindersHelperTest) TestListPendingAllNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&FollowUpRemindersHelper{}).ListPendingAll()
		return err
	})
}

func (test *FollowUpRemindersHelperTest) TestFindByIdNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, _, err := (&FollowUpRemindersHelper{}).FindById("reminder-1")
		return err
	})
}

func (test *FollowUpRemindersHelperTest) TestFindActiveForEmailNeedsTheDatabase() {
	Assert.Throws(func() error {
		_, err := (&FollowUpRemindersHelper{}).FindActiveForEmail("ada@example.com", "email-1")
		return err
	})
}

func (test *FollowUpRemindersHelperTest) TestUpdateNeedsTheDatabase() {
	Assert.Throws(func() error {
		err := (&FollowUpRemindersHelper{}).Update("doc-1", map[string]any{"status": "done"})
		return err
	})
}
