import (
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

// @Service
type OutboxRouteService struct {
	// @Needs
	relay *OutboxRelayService
}

func (service *OutboxRouteService) Relay(authorization string) (outbox.Report, error) {
	return service.relayUnless(service.installWide(authorization))
}

func (service *OutboxRouteService) Dead(authorization string) ([]DeadEvent, error) {
	return service.deadUnless(service.installWide(authorization))
}

func (service *OutboxRouteService) Retry(authorization string, request RetryRequest) error {
	return service.retryUnless(service.installWide(authorization), request)
}

func (service *OutboxRouteService) installWide(authorization string) error {
	return service.installWideOnly(service.fromInstallWide(authorization))
}

func (service *OutboxRouteService) fromInstallWide(authorization string) bool {
	caller, failure := oidc.Configured().Identify(authorization)
	return failure == nil && oidc.InstallWide{}.Matches(caller.DomainId)
}

func (service *OutboxRouteService) installWideOnly(fromInstallWide bool) error {
	if fromInstallWide {
		return nil
	}
	return Forbidden{}.NotInstallWide()
}

func (service *OutboxRouteService) relayUnless(refused error) (outbox.Report, error) {
	if refused != nil {
		return outbox.Report{}, refused
	}
	return service.relay.RunOnce()
}

func (service *OutboxRouteService) deadUnless(refused error) ([]DeadEvent, error) {
	if refused != nil {
		return nil, refused
	}
	return service.relay.DeadEvents()
}

func (service *OutboxRouteService) retryUnless(refused error, request RetryRequest) error {
	if refused != nil {
		return refused
	}
	return service.relay.RetryDead(request)
}
