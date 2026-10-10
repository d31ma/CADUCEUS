import "net/http"

// TestSeedUserController answers /test/seed/user, which provisions one
// mailbox and signs it in. It answers only when the deployment enables the
// test routes.
// @RequestRoute("/test/seed/user")
// @Public
type TestSeedUserController struct {
	// @Needs
	usersHelper *UsersHelper
}

// PostUser provisions the mailbox and answers its tokens.
// @PostRoute
func (controller *TestSeedUserController) PostUser(request *http.Request) *http.Response {
	return TestSeedUserRoute{Users: controller.usersHelper}.Seed(request)
}
