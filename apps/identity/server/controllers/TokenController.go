// @RequestRoute("/api/v1/identity")
// @Public
type TokenController struct {
	// @Needs
	tokens *TokenService
}

// @GetRoute("/jwks")
func (controller *TokenController) GetJwks() (KeySet, error) {
	return controller.tokens.KeySet()
}

// @PostRoute("/service-token")
func (controller *TokenController) PostServiceToken(
	// @FromBody
	request ServiceTokenRequest,
) (ServiceToken, error) {
	return controller.tokens.ServiceToken(request)
}
