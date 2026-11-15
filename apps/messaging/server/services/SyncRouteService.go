import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

// @Service
type SyncRouteService struct {
	// @Needs
	callers *CallerService
	// @Needs
	messaging *MessagingService
	// @Needs
	wire *WireService
}

func (service *SyncRouteService) Sync(authorization string, request SyncRequest) (SyncResponse, error) {
	caller, refused := service.callers.ChatCaller(authorization)
	answers, err := service.sync(caller, service.wire.SyncCursors(request), refused)
	return service.wire.SyncResponse(answers), err
}

func (service *SyncRouteService) sync(caller oidc.Caller, cursors []core.SyncCursor, failure error) ([]core.ConversationSync, error) {
	if failure != nil {
		return nil, failure
	}
	return service.messaging.Sync(caller, cursors)
}
