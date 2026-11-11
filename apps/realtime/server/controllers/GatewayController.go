import (
	"errors"
	"os"

	"mesail.dev/packages/realtime"
)

// @RequestRoute("/api/v1/realtime/gateway")
// @Allow(Gateway)
type GatewayController struct {
	// @Needs
	dataSource DataSource
}

// @PostRoute("/connect")
// @ResponseStatus(204)
func (controller *GatewayController) PostConnect(
	// @FromBody
	request GatewayConnect,
) error {
	return controller.failure(controller.gateway().Connect(realtime.GatewayConnect{ConnectionId: request.ConnectionId, Kind: request.Kind}))
}

// @PostRoute("/message")
func (controller *GatewayController) PostMessage(
	// @FromBody
	request GatewayMessage,
) (GatewayReply, error) {
	reply, failed := controller.gateway().Message(realtime.GatewayMessage{ConnectionId: request.ConnectionId, DomainId: request.DomainId, Body: request.Body})
	return GatewayReply{DomainId: reply.DomainId, ExpiresAt: reply.ExpiresAt}, controller.failure(failed)
}

// @PostRoute("/disconnect")
// @ResponseStatus(204)
func (controller *GatewayController) PostDisconnect(
	// @FromBody
	request GatewayDisconnect,
) error {
	return controller.failure(controller.gateway().Disconnect(realtime.GatewayDisconnect{ConnectionId: request.ConnectionId, DomainId: request.DomainId}))
}

func (controller *GatewayController) gateway() realtime.Gateway {
	return realtime.DeskFactory{Environment: os.Getenv}.Gateway(StorageUtils{}.From(controller.dataSource))
}

func (controller *GatewayController) failure(failed error) error {
	invalid := realtime.Invalid{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &invalid):
		return &InvalidRequest{Field: invalid.Field, Reason: invalid.Reason}
	case errors.As(failed, &realtime.Missing{}):
		return &NotFound{}
	case errors.Is(failed, realtime.Unauthenticated{}):
		return &Unauthenticated{}
	}
	return &Unavailable{}
}
