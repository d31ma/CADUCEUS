import "net/http"

// UsersMePreferencesController answers /users/me/preferences: the caller's
// display and reading preferences.
// @RequestRoute("/users/me/preferences")
// @SignedIn
type UsersMePreferencesController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	usersHelper *UsersHelper
}

// PutPreferences updates the caller's preferences.
// @PutRoute
func (controller *UsersMePreferencesController) PutPreferences(request *http.Request) *http.Response {
	return UsersPreferencesRoute{Access: controller.accessHelper, Users: controller.usersHelper}.Update(request)
}

// GetPreferences answers the caller's preferences.
// @GetRoute
func (controller *UsersMePreferencesController) GetPreferences(request *http.Request) *http.Response {
	return UsersPreferencesRoute{Access: controller.accessHelper, Users: controller.usersHelper}.Read(request)
}

