import (
	"mesail.dev/packages/channels"
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

func (service *OutboxService) Relay() (channels.SweepReport, error) {
	report, err := service.desk().RunOnce()
	return report, service.failures.OutboxError(err)
}

func (service *OutboxService) Pending() (OutboxCount, error) {
	waiting, err := service.desk().Waiting()
	return OutboxCount{Documents: waiting}, service.failures.OutboxError(err)
}

func (service *OutboxService) Dead() ([]channels.DeadEvent, error) {
	dead, err := service.desk().DeadEvents()
	return dead, service.failures.OutboxError(err)
}

func (service *OutboxService) Retry(request RetryRequest) error {
	err := service.desk().RetryDead(request.Collection, request.DocumentId, request.EventId, request.Subscriber)
	return service.failures.OutboxError(err)
}

func (service *OutboxService) desk() channels.OutboxDesk {
	return channels.Assembly{Source: service.dataSource, Pusher: service.pusher}.Desk()
}
