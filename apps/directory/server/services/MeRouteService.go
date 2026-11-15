import "mesail.dev/packages/oidc"

// @Service
type MeRouteService struct {
	// @Needs
	callers *CallerService
	// @Needs
	people *PersonService
}

func (service *MeRouteService) SetDefault(authorization string, request DefaultDomain) error {
	caller, refused := service.callers.Caller(authorization)
	return service.defaultUnless(refused, caller, request)
}

func (service *MeRouteService) defaultUnless(refused error, caller oidc.Caller, request DefaultDomain) error {
	if refused != nil {
		return refused
	}
	return service.people.SetDefault(caller, request)
}
