// @RequestRoute("/api/v1/directory/me")
// @SignedIn
type MeController struct {
	// @Needs
	me *MeRouteService
}

// @PutRoute("/default-domain")
// @ResponseStatus(204)
func (controller *MeController) PutDefaultDomain(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request DefaultDomain,
) error {
	return controller.me.SetDefault(authorization, request)
}
