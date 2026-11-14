import (
	"os"

	"mesail.dev/packages/realtime"
)

// @Service
type GatewayService struct {
	// @Needs
	dataSource DataSource
	// @Needs
	failures *ErrorMappingService
}

func (service *GatewayService) Connect(request GatewayConnect) error {
	failed := service.gateway().Connect(realtime.GatewayConnect{ConnectionId: request.ConnectionId, Kind: request.Kind})
	return service.failures.GatewayError(failed)
}

func (service *GatewayService) Message(request GatewayMessage) (GatewayReply, error) {
	reply, failed := service.gateway().Message(realtime.GatewayMessage{ConnectionId: request.ConnectionId, DomainId: request.DomainId, Body: request.Body})
	return GatewayReply{DomainId: reply.DomainId, ExpiresAt: reply.ExpiresAt}, service.failures.GatewayError(failed)
}

func (service *GatewayService) Disconnect(request GatewayDisconnect) error {
	failed := service.gateway().Disconnect(realtime.GatewayDisconnect{ConnectionId: request.ConnectionId, DomainId: request.DomainId})
	return service.failures.GatewayError(failed)
}

func (service *GatewayService) gateway() realtime.Gateway {
	return realtime.DeskFactory{Environment: os.Getenv}.Gateway(StorageUtils{}.From(service.dataSource))
}
