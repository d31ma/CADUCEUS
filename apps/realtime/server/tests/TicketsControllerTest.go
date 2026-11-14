import "errors"

// @Test(GatewayTestSetup, {"partition":"tests"})
type TicketsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *TicketsControllerTest) TestPostTicketRefusesAMissingToken() {
	GatewayTestSetup{}.Realm("http://127.0.0.1:1")
	_, failure := test.controller().PostTicket("")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(failure, &refused), "a ticket without a token")
	Assert.Equals(5011, refused.Number())
}

func (test *TicketsControllerTest) TestPostTicketGivesAMemberATicket() {
	setup := GatewayTestSetup{}
	realm := setup.Realm("http://127.0.0.1:1")
	ticket, failure := test.controller().PostTicket("Bearer " + setup.Token(realm, []string{"mesail-member"}))
	Assert.IsTrue(failure == nil, "a member got no ticket")
	Assert.Equals(43, len(ticket.Ticket))
}

func (test *TicketsControllerTest) TestPostTicketRefusesAServiceToken() {
	setup := GatewayTestSetup{}
	realm := setup.Realm("http://127.0.0.1:1")
	_, failure := test.controller().PostTicket("Bearer " + setup.ServiceToken(realm))
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(failure, &refused), "a service token got a ticket")
	Assert.Equals(5012, refused.Number())
}

func (test *TicketsControllerTest) controller() *TicketsController {
	return &TicketsController{tickets: &TicketService{failures: &ErrorMappingService{}}}
}
