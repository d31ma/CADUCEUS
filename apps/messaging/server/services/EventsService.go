import (
	"slices"
	"strings"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/inbound"
	"mesail.dev/packages/oidc"
)

// @Service
type EventsService struct {
	// @Needs
	core *CoreService
	// @Needs
	seen *SeenEventsService
	// @Needs
	failures *FailureService
}

func (service *EventsService) Receive(caller oidc.Caller, envelope events.Envelope) (events.Receipt, error) {
	receipt, err := service.received(caller, envelope, service.domainRefusal(caller, envelope.DomainId))
	return receipt, service.failures.Known(err)
}

func (service *EventsService) received(caller oidc.Caller, envelope events.Envelope, failure error) (events.Receipt, error) {
	if failure != nil {
		return events.Receipt{}, failure
	}
	return service.receiver().Receive(caller, envelope)
}

func (service *EventsService) receiver() events.Receiver {
	handler := inbound.NewEventHandler(service.core.Stores(), service.core.Ports())
	return events.NewReceiver(events.Services{}.Messaging(), service.seen, handler)
}

func (service *EventsService) domainRefusal(caller oidc.Caller, domainId string) error {
	if slices.Contains(caller.Roles, service.serviceRole()) && !strings.EqualFold(caller.DomainId, domainId) {
		return &core.NotFound{Message: "no such domain"}
	}
	return nil
}

func (service *EventsService) serviceRole() string {
	return "Service"
}
