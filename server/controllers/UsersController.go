import "net/http"

// UsersController answers /users: the mailbox records of the domains the
// caller administers.
// @RequestRoute("/users")
// @SignedIn
type UsersController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	usersHelper *UsersHelper
}

// PostUser stores a new user.
// @PostRoute
func (controller *UsersController) PostUser(request *http.Request) *http.Response {
	return UsersRoute{Access: controller.accessHelper, Users: controller.usersHelper}.Create(request)
}

// GetUsers answers every user.
// @GetRoute
func (controller *UsersController) GetUsers(request *http.Request) *http.Response {
	return UsersRoute{Access: controller.accessHelper, Users: controller.usersHelper}.List(request)
}

