import "strings"

// UserAdministrationService lets an administrator list and add users, and
// shows the operator command that issues a user's first credentials. That
// command needs the deployment's key boundary, so the browser cannot run it.
// @Service
type UserAdministrationService struct {
	// @Needs
	usersClient               *UsersClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	issued                    map[string]bool
	Administrator             bool
	Users                     []UserAccount
	// @SessionState
	Adding bool
	// @SessionState
	Email string
	// @SessionState
	Domains string
}

// @OnStart
func (service *UserAdministrationService) Start() {
	service.issued = map[string]bool{}
	service.Users = []UserAccount{}
}

// @OnEvent({"replay":true})
func (service *UserAdministrationService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
	service.Administrator = ChoiceUtils{}.All(changed.SignedIn, changed.Role == "admin")
	refused := false
	FlowUtils{}.When(service.Administrator, func() { refused = service.reload() })()
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "users", Refused: refused}, false)
}

func (service *UserAdministrationService) Rows() []UserRow {
	return service.rows()
}

func (service *UserAdministrationService) ToggleForm() {
	service.Adding = !service.Adding
}

func (service *UserAdministrationService) Add() error {
	FlowUtils{}.When(ChoiceUtils{}.All(service.Email != "", service.Domains != ""), service.add)()
	return nil
}

func (service *UserAdministrationService) IssueRecovery(email string) {
	service.issued[email] = true
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Run the operator command shown to issue credentials."}, false)
}

func (service *UserAdministrationService) add() {
	domains := strings.Split(service.Domains, ",")
	for index := range domains {
		domains[index] = strings.TrimSpace(domains[index])
	}
	_, failure := service.usersClient.AddUser(service.authorization, map[string]any{"email": service.Email, "domains": PreferenceUtils{}.Without(domains, ""), "role": "viewer"})
	FlowUtils{}.Either(failure == nil, service.added, func() {
		service.toast(HTTPUtils{}.Message(failure, "Failed to add user.", "Network error."))
	})()
}

func (service *UserAdministrationService) added() {
	service.Adding = false
	service.Email = ""
	service.Domains = ""
	service.reload()
	service.toast("User added. Issue an enrollment code when they are ready.")
}

func (service *UserAdministrationService) toast(message string) {
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (service *UserAdministrationService) rows() []UserRow {
	rows := []UserRow{}
	for _, user := range service.Users {
		rows = append(rows, UserRow{
			Email: user.Email, Role: user.Role, HasRecovery: service.issued[user.Email],
			Note:            "Run this on the server. It prints a recovery code set once; hand it to the mailbox owner out of band.",
			OperatorCommand: "caduceus auth credentials --address " + user.Email + " --password '<generated>'",
		})
	}
	return rows
}

// reload reads the users and says whether the server refused the read.
func (service *UserAdministrationService) reload() bool {
	users, failure := service.usersClient.Users(service.authorization)
	service.Users = append([]UserAccount{}, users...)
	return HTTPUtils{}.Refused(failure)
}
