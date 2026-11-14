import "mesail.dev/packages/oidc"

// @Service
type AttachmentRouteService struct {
	// @Needs
	callers *CallerService
	// @Needs
	messaging *MessagingService
}

func (service *AttachmentRouteService) Access(authorization string, attachmentId string) error {
	caller, refused := service.callers.ChatCaller(authorization)
	return service.access(caller, attachmentId, refused)
}

func (service *AttachmentRouteService) access(caller oidc.Caller, attachmentId string, failure error) error {
	if failure != nil {
		return failure
	}
	return service.messaging.AttachmentAccess(caller, attachmentId)
}
