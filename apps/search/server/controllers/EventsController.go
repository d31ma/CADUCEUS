import (
	"errors"
	"os"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/search"
)

// @RequestRoute("/api/v1/search/events")
// @Allow(Service)
type EventsController struct {
	// @Needs
	dataSource DataSource
}

// @PostRoute
func (controller *EventsController) PostEvent(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	envelope EventEnvelope,
) (EventReceipt, error) {
	caller, _ := oidc.Configured().Identify(authorization)
	receipt := EventReceipt{}
	service := search.ServiceFactory{Environment: os.Getenv}.Service(controller.dataSource)
	failure := service.ReceiveInto(caller, envelope, &receipt)
	return receipt, controller.failure(failure)
}

func (controller *EventsController) failure(failed error) error {
	refused := search.InvalidRequest{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &refused):
		return &InvalidRequest{Message: refused.Message, Field: refused.Field}
	case errors.As(failed, &search.Forbidden{}):
		return &Forbidden{Message: failed.Error()}
	}
	return &Unavailable{Message: failed.Error()}
}
