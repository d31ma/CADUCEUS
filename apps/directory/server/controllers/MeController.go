import "mesail.dev/packages/oidc"

// @RequestRoute("/api/v1/directory/me")
// @SignedIn
type MeController struct {
	// @Needs
	callers *CallerService
	// @Needs
	people *PersonService
}

// @PutRoute("/default-domain")
// @ResponseStatus(204)
func (controller *MeController) PutDefaultDomain(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request DefaultDomain,
) error {
	caller, refused := controller.callers.Caller(authorization)
	return controller.defaultUnless(refused, caller, request)
}

func (controller *MeController) defaultUnless(refused error, caller oidc.Caller, request DefaultDomain) error {
	if refused != nil {
		return refused
	}
	return controller.people.SetDefault(caller, request)
}
