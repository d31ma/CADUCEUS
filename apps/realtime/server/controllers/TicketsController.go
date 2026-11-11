import (
	"errors"
	"os"

	"mesail.dev/packages/realtime"
)

// @RequestRoute("/api/v1/realtime/tickets")
// @SignedIn
type TicketsController struct {
	// @Needs
	dataSource DataSource
}

// @PostRoute
func (controller *TicketsController) PostTicket(
	// @FromHeader("authorization")
	authorization string,
) (Ticket, error) {
	ticket, failed := controller.tickets().Issue(authorization)
	return Ticket{Ticket: ticket.Ticket, ExpiresAt: ticket.ExpiresAt}, controller.failure(failed)
}

func (controller *TicketsController) tickets() realtime.Tickets {
	return realtime.DeskFactory{Environment: os.Getenv}.Tickets(StorageUtils{}.From(controller.dataSource))
}

func (controller *TicketsController) failure(failed error) error {
	switch {
	case failed == nil:
		return nil
	case errors.Is(failed, realtime.Unauthenticated{}):
		return &Unauthenticated{}
	}
	return &Unavailable{}
}
