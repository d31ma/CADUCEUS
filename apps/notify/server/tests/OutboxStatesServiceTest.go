// @Test(NotifyTestSetup, {"partition":"tests"})
type OutboxStatesServiceTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxStatesServiceTest) TestPendingCountMakesThePendingState() {
	states := &OutboxStatesService{}
	Assert.Equals(states.Waiting(), states.Pending(2))
	Assert.Equals("settled", states.Pending(0))
}

func (test *OutboxStatesServiceTest) TestDeadCountMakesTheDeadState() {
	states := &OutboxStatesService{}
	Assert.Equals(states.Failed(), states.Dead(1))
	Assert.Equals("none", states.Dead(0))
}
