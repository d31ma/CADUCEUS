import "errors"

// @Test(GatewayTestSetup, {"partition":"tests"})
type TicketsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *TicketsControllerTest) TestPostTicketRefusesAMissingToken() {
	GatewayTestSetup{}.Realm("http://127.0.0.1:1")
	_, failure := (&TicketsController{}).PostTicket("")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(failure, &refused), "a ticket without a token")
}

func (test *TicketsControllerTest) TestPostTicketGivesAMemberATicket() {
	setup := GatewayTestSetup{}
	realm := setup.Realm("http://127.0.0.1:1")
	ticket, failure := (&TicketsController{}).PostTicket("Bearer " + setup.Token(realm, []string{"mesail-member"}))
	Assert.IsTrue(failure == nil, "a member got no ticket")
	Assert.Equals(43, len(ticket.Ticket))
}

func (test *TicketsControllerTest) TestPostTicketRefusesAServiceToken() {
	setup := GatewayTestSetup{}
	realm := setup.Realm("http://127.0.0.1:1")
	_, failure := (&TicketsController{}).PostTicket("Bearer " + setup.ServiceToken(realm))
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(failure, &refused), "a service token got a ticket")
}
