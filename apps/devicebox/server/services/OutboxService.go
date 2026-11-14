import (
	"mesail.dev/packages/devicebox"
	"mesail.dev/packages/outbox"
)

// @Service
type OutboxService struct {
	// @Needs
	dataSource DataSource
	// @Needs
	failures *ErrorMappingService
	pusher   outbox.Pusher
}

func (service *OutboxService) Relay() (outbox.Report, error) {
	report, err := service.desk().RunOnce()
	return report, service.failures.Outbox(err)
}

func (service *OutboxService) Dead() ([]devicebox.DeadEvent, error) {
	dead, err := service.desk().DeadEvents()
	return dead, service.failures.Outbox(err)
}

func (service *OutboxService) Retry(request RetryRequest) error {
	ref := outbox.Ref{Collection: request.Collection, Id: request.DocumentId}
	err := service.desk().RetryDead(ref, request.EventId, request.Subscriber)
	return service.failures.Outbox(err)
}

func (service *OutboxService) desk() devicebox.OutboxDesk {
	return devicebox.Assembly{Source: service.dataSource, Pusher: service.pusher}.Desk()
}
