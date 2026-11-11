// @Service
type TicketService struct {
	// @Needs
	dataSource DataSource
}

func (service *TicketService) Issue(grant AuthGrant) (Ticket, error) {
	return Ticket{}, nil
}

func (service *TicketService) Redeem(ticket string) (AuthGrant, error) {
	return AuthGrant{}, &Unauthenticated{}
}
