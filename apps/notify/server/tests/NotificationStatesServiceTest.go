// @Test(NotifyTestSetup, {"partition":"tests"})
type NotificationStatesServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationStatesServiceTest) TestPushStates() {
	states := &NotificationStatesService{}
	Assert.Equals("pending", states.PushPending())
	Assert.Equals("sent", states.PushSent())
	Assert.Equals("skipped", states.PushSkipped())
	Assert.Equals("failed", states.PushFailed())
}

func (test *NotificationStatesServiceTest) TestSummaryStates() {
	states := &NotificationStatesService{}
	Assert.Equals("waiting", states.SummaryWaiting())
	Assert.Equals("none", states.SummaryNone())
}
