// @RequestRoute("/api/v1/directory/recipients")
// @SignedIn
type RecipientsController struct {
	// @Needs
	admission *AdmissionService
	// @Needs
	recipients *RecipientService
}

// @PostRoute("/resolve")
func (controller *RecipientsController) PostResolve(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request ResolveRequest,
) (ResolveResponse, error) {
	return controller.recipients.Resolve(controller.admission.Admit(authorization), request)
}
