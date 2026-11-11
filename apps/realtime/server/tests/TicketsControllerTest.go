// @Test(HealthTestSetup, {"partition":"tests"})
type TicketsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *TicketsControllerTest) TestPostTicketRefusesAMissingToken() {
	_, failure := (&TicketsController{}).PostTicket("")
	Assert.IsTrue(failure != nil, "a ticket without a token")
}
