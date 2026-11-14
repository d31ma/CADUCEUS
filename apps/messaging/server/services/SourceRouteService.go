import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

// @Service
type SourceRouteService struct {
	// @Needs
	callers *CallerService
	// @Needs
	messaging *MessagingService
	// @Needs
	wire *WireService
}

func (service *SourceRouteService) Source(authorization string, messageId string) (MessageSource, error) {
	caller, refused := service.callers.ServiceCaller(authorization)
	source, err := service.source(caller, messageId, refused)
	return service.wire.MessageSource(source), err
}

func (service *SourceRouteService) source(caller oidc.Caller, messageId string, failure error) (core.MessageSource, error) {
	if failure != nil {
		return core.MessageSource{}, failure
	}
	return service.messaging.Source(caller, messageId)
}
