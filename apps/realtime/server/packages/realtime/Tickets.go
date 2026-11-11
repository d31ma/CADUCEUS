package realtime

import (
	"time"

	"mesail.dev/packages/oidc"
)

type Tickets struct {
	Tokens TokenChecker
	Book   TicketBook
	Now    func() time.Time
}

func (tickets Tickets) Issue(authorization string) (Ticket, error) {
	token, err := oidc.BearerToken{}.FromHeader(authorization)
	if err != nil {
		return Ticket{}, Unauthenticated{}
	}
	grant, err := tickets.Tokens.Grant(token, tickets.Now())
	if err != nil {
		return Ticket{}, err
	}
	return tickets.Book.Issue(grant)
}
