// @Service
type RecipientRouteService struct {
	// @Needs
	admission *AdmissionService
	// @Needs
	recipients *RecipientService
}

func (service *RecipientRouteService) Resolve(authorization string, request ResolveRequest) (ResolveResponse, error) {
	return service.recipients.Resolve(service.admission.Admit(authorization), request)
}
