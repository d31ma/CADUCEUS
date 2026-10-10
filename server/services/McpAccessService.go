// McpAccessService keeps each mailbox's MCP configuration and the hash-only
// capability an MCP client signs in with.
// @Service
type McpAccessService struct {
	// @Needs
	mcpAccessTokensHelper *McpAccessTokensHelper
	// @Needs
	usersHelper *UsersHelper
	// @Needs
	fyloHelper *FyloHelper
}

// Config answers the owner's configuration, or nil without a user record.
func (service *McpAccessService) Config(ownerEmail string) (map[string]any, error) {
	return service.access().Config(ownerEmail)
}

// UpdateConfig merges a patch into the owner's configuration; a refused
// change fails with an *McpPreferenceError.
func (service *McpAccessService) UpdateConfig(ownerEmail string, patch any) (map[string]any, error) {
	return service.access().UpdateConfig(ownerEmail, patch)
}

// Rotate replaces the owner's capability and answers the new one once.
func (service *McpAccessService) Rotate(ownerEmail string) (map[string]any, error) {
	return service.access().Rotate(ownerEmail)
}

// Revoke revokes the owner's capability and reports whether one was active.
func (service *McpAccessService) Revoke(ownerEmail string) (bool, error) {
	return service.access().Revoke(ownerEmail)
}

// Authenticate answers the principal of an MCP bearer token, or nil.
func (service *McpAccessService) Authenticate(token string) (*McpPrincipal, error) {
	return service.access().Authenticate(token)
}

func (service *McpAccessService) access() McpAccess {
	return McpAccess{Tokens: service.mcpAccessTokensHelper, Users: service.usersHelper, Fylo: service.fyloHelper}
}
