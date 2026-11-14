// @RequestRoute("/api/v1/realtime/tickets")
// @SignedIn
type TicketsController struct {
	// @Needs
	tickets *TicketService
}

// @PostRoute
func (controller *TicketsController) PostTicket(
	// @FromHeader("authorization")
	authorization string,
) (Ticket, error) {
	return controller.tickets.Issue(authorization)
}
