// @Service
type AliasRouteService struct {
	// @Needs
	admission *AdmissionService
	// @Needs
	aliases *AliasService
}

func (service *AliasRouteService) Add(authorization string, request CreateAliasRequest) (Address, error) {
	return service.aliases.Add(service.admission.Admit(authorization), request)
}
