// @Test(NotificationTestSetup, {"partition":"tests"})
type NotificationKindServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationKindServiceTest) TestTextNamesEachKind() {
	kinds := &NotificationKindService{}
	Assert.Equals("You were mentioned", kinds.Text("mention"))
	Assert.Equals("New direct message", kinds.Text("dm"))
	Assert.Equals("New reply in a thread", kinds.Text("thread_reply"))
	Assert.Equals("New email", kinds.Text("email"))
	Assert.Equals("New activity", kinds.Text("something"))
}

func (test *NotificationKindServiceTest) TestTabs() {
	kinds := &NotificationKindService{}
	Assert.Equals([]string{"all", "mention", "dm", "thread_reply", "email"}, kinds.TabNames())
	Assert.Equals("all", kinds.AllTab())
	Assert.Equals("Direct messages", kinds.TabLabel("dm"))
	Assert.Equals("Threads", kinds.TabLabel("thread_reply"))
	Assert.Equals("other", kinds.TabLabel("other"))
}
