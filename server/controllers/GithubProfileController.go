import "net/http"

// @RequestRoute("/github/profile")
// @SignedIn
type GithubProfileController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	contactsHelper *ContactsHelper
	// @Needs
	emailsHelper *EmailsHelper
	// @Needs
	gitHubProfilesService *GitHubProfilesService
	// @Needs
	usersHelper *UsersHelper
}

// @GetRoute
func (controller *GithubProfileController) GetProfile(request *http.Request) *http.Response {
	route := GithubProfileRoute{
		Access: controller.accessHelper, Contacts: controller.contactsHelper, Emails: controller.emailsHelper,
		GitHub: controller.gitHubProfilesService, Users: controller.usersHelper,
	}
	return route.Find(request)
}
