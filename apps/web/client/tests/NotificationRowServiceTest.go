// @Test(NotificationTestSetup, {"partition":"tests"})
type NotificationRowServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationRowServiceTest) rows() *NotificationRowService {
	NotificationTestSetup{}.answerBrowser()
	return NotificationTestSetup{}.rows()
}

func (test *NotificationRowServiceTest) TestTabsAndCounts() {
	rows := test.rows()
	items := NotificationTestSetup{}.firstItems()
	tabs := rows.Tabs(items, "dm")
	Assert.Equals("all", rows.AllTab())
	Assert.Equals("activity-tab-dm", rows.TabId("dm"))
	Assert.Equals(5, len(tabs))
	Assert.IsTrue(tabs[2].Selected)
	Assert.Equals(3, rows.Count(items, "all"))
	Assert.Equals(0, rows.Count(items, "thread_reply"))
}

func (test *NotificationRowServiceTest) TestDaysKeepReadRows() {
	days := test.rows().Days(NotificationTestSetup{}.firstItems(), "thread_reply")
	Assert.Equals(1, len(days))
	Assert.IsTrue(!days[0].Rows[0].Unread)
	Assert.Equals("Mark as read: New reply in a thread, 09:31", days[0].Rows[0].MarkLabel)
}

func (test *NotificationRowServiceTest) TestUnreadIdsStopAtTheLimit() {
	Assert.Equals([]string{"n1", "n2"}, test.rows().UnreadIds(NotificationTestSetup{}.firstItems(), 2))
}

func (test *NotificationRowServiceTest) TestFreshNeedsAKnownList() {
	rows := test.rows()
	items := NotificationTestSetup{}.firstItems()
	seen := rows.Seen(items[:1], nil)
	Assert.Equals(0, len(rows.Fresh(items, seen, false)))
	Assert.Equals(2, len(rows.Fresh(items, seen, true)))
}

func (test *NotificationRowServiceTest) TestReadOnlyWhenSaved() {
	rows := test.rows()
	items := NotificationTestSetup{}.firstItems()
	Assert.Equals("2026-11-10T11:00:00Z", rows.Read(items, "n1", "")[0].ReadAt)
	Assert.Equals("", rows.Read(items, "n1", "error")[0].ReadAt)
}
