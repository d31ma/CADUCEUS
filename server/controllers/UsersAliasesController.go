import "net/http"

// UsersAliasesController answers /users/aliases: the extra addresses of the
// caller's own mailbox.
// @RequestRoute("/users/aliases")
// @SignedIn
type UsersAliasesController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	usersHelper *UsersHelper
}

// PostAlias adds an alias.
// @PostRoute
func (controller *UsersAliasesController) PostAlias(request *http.Request) *http.Response {
	aliases := controller.route()
	return aliases.Add(request)
}

// DeleteAlias removes an alias.
// @DeleteRoute
func (controller *UsersAliasesController) DeleteAlias(request *http.Request) *http.Response {
	aliases := controller.route()
	return aliases.Remove(request)
}

// GetAliases answers the caller's aliases.
// @GetRoute
func (controller *UsersAliasesController) GetAliases(request *http.Request) *http.Response {
	aliases := controller.route()
	return aliases.List(request)
}

func (controller *UsersAliasesController) route() UsersAliasesRoute {
	return UsersAliasesRoute{Access: controller.accessHelper, Users: controller.usersHelper}
}
