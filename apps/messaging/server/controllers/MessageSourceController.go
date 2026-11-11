import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

// @RequestRoute("/api/v1/messaging/messages")
// @Allow(Service)
type MessageSourceController struct {
	// @Needs
	callers *CallerService
	// @Needs
	messaging *MessagingService
	// @Needs
	wire *WireService
}

// @GetRoute("/{messageId}/source")
func (controller *MessageSourceController) GetSource(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath
	messageId string,
) (MessageSource, error) {
	caller, refused := controller.callers.ServiceCaller(authorization)
	source, err := controller.source(caller, messageId, refused)
	return controller.wire.MessageSource(source), err
}

func (controller *MessageSourceController) source(caller oidc.Caller, messageId string, failure error) (core.MessageSource, error) {
	if failure != nil {
		return core.MessageSource{}, failure
	}
	return controller.messaging.Source(caller, messageId)
}
