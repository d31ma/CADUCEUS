// @RequestRoute("/api/v1/directory/aliases")
// @Allow(DomainAdmin)
type AliasesController struct {
	// @Needs
	aliases *AliasRouteService
}

// @PostRoute
// @ResponseStatus(201)
func (controller *AliasesController) PostAlias(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request CreateAliasRequest,
) (Address, error) {
	return controller.aliases.Add(authorization, request)
}
