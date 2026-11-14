import (
	"os"

	"mesail.dev/packages/realtime"
)

// @Service
type TicketService struct {
	// @Needs
	dataSource DataSource
	// @Needs
	failures *ErrorMappingService
}

func (service *TicketService) Issue(authorization string) (Ticket, error) {
	ticket, failed := service.tickets().Issue(authorization)
	return Ticket{Ticket: ticket.Ticket, ExpiresAt: ticket.ExpiresAt}, service.failures.TicketError(failed)
}

func (service *TicketService) tickets() realtime.Tickets {
	return realtime.DeskFactory{Environment: os.Getenv}.Tickets(StorageUtils{}.From(service.dataSource))
}
