package realtime

import (
	"io"
	"time"
)

type TicketBook struct {
	Storage Storage
	Now     func() time.Time
	Random  io.Reader
}

func (book TicketBook) Issue(grant Grant) (Ticket, error) {
	return Ticket{}, nil
}

func (book TicketBook) Redeem(ticket string) (Grant, error) {
	return Grant{}, Unauthenticated{}
}
