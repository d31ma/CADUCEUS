// @RequestRoute("/api/v1/directory/recipients")
// @SignedIn
type RecipientsController struct {
	// @Needs
	recipients *RecipientRouteService
}

// @PostRoute("/resolve")
func (controller *RecipientsController) PostResolve(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request ResolveRequest,
) (ResolveResponse, error) {
	return controller.recipients.Resolve(authorization, request)
}
