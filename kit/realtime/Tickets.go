package realtime

import "time"

type Tickets struct {
	Tokens TokenChecker
	Book   TicketBook
	Now    func() time.Time
}

func (tickets Tickets) Issue(authorization string) (Ticket, error) {
	return Ticket{}, Unauthenticated{}
}
