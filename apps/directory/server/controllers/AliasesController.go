// @RequestRoute("/api/v1/directory/aliases")
// @Allow(DomainAdmin)
type AliasesController struct {
	// @Needs
	admission *AdmissionService
	// @Needs
	aliases *AliasService
}

// @PostRoute
// @ResponseStatus(201)
func (controller *AliasesController) PostAlias(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request CreateAliasRequest,
) (Address, error) {
	return controller.aliases.Add(controller.admission.Admit(authorization), request)
}
