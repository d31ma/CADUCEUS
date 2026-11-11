import (
	"errors"

	"mesail.dev/packages/channels"
	"mesail.dev/packages/outbox"
)

// @RequestRoute("/api/v1/channels/outbox")
// @Allow(Service)
type OutboxController struct {
	// @Needs
	dataSource DataSource
	pusher     outbox.Pusher
}

// @PostRoute("/relay")
func (controller *OutboxController) PostRelay() (channels.SweepReport, error) {
	report, err := controller.desk().RunOnce()
	return report, controller.failure(err)
}

// @GetRoute("/pending")
func (controller *OutboxController) GetPending() (OutboxCount, error) {
	waiting, err := controller.desk().Waiting()
	return OutboxCount{Documents: waiting}, controller.failure(err)
}

// @GetRoute("/dead")
func (controller *OutboxController) GetDead() ([]channels.DeadEvent, error) {
	dead, err := controller.desk().DeadEvents()
	return dead, controller.failure(err)
}

// @PostRoute("/dead/requeue")
// @ResponseStatus(204)
func (controller *OutboxController) PostRetry(
	// @FromBody
	request RetryRequest,
) error {
	err := controller.desk().RetryDead(request.Collection, request.DocumentId, request.EventId, request.Subscriber)
	return controller.failure(err)
}

func (controller *OutboxController) desk() channels.OutboxDesk {
	return channels.Assembly{Source: controller.dataSource, Pusher: controller.pusher}.Desk()
}

func (controller *OutboxController) failure(failed error) error {
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, new(*channels.NotFound)):
		return &NotFound{Message: failed.Error()}
	}
	return &Unavailable{Message: "the outbox is not available"}
}
