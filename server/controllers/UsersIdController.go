import "net/http"

// UsersIdController answers /users/{id}: one user, named by an address.
// @RequestRoute("/users/{id}")
// @SignedIn
type UsersIdController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	usersHelper *UsersHelper
}

// DeleteUser deletes the user.
// @DeleteRoute
func (controller *UsersIdController) DeleteUser(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return UsersRoute{Access: controller.accessHelper, Users: controller.usersHelper}.Delete(id, request)
}
