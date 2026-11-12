import "mesail.dev/packages/oidc"

// @Service
type EventService struct {
	// @Needs
	indexes *IndexService
	// @Needs
	failures *FailureService
}

func (service *EventService) Receive(authorization string, envelope EventEnvelope) (EventReceipt, error) {
	caller, _ := oidc.Configured().Identify(authorization)
	receipt := EventReceipt{}
	failed := service.indexes.Index().ReceiveInto(caller, envelope, &receipt)
	return receipt, service.failures.Translated(failed)
}
