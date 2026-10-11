// PasswordChangeService sets or rotates the account password after a fresh
// step-up. The new password stays in memory only.
// @Service
type PasswordChangeService struct {
	// @Needs
	authClient                *AuthClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	NewPassword               string
	ConfirmPassword           string
	Saving                    bool
	Error                     string
}

// @OnEvent({"replay":true})
func (service *PasswordChangeService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
}

func (service *PasswordChangeService) Update() {
	problem := service.problem(ChoiceUtils{}.All(service.ConfirmPassword != "", service.ConfirmPassword != service.NewPassword))
	service.Error = problem
	FlowUtils{}.When(problem == "", service.prove)()
}

// @OnEvent
func (service *PasswordChangeService) Proven(completed StepUpCompleted) {
	FlowUtils{}.When(completed.Purpose == service.purpose(), func() { service.finish(completed.Transaction) })()
}

func (service *PasswordChangeService) prove() {
	service.Saving = true
	service.applicationEventPublisher.PublishEvent(StepUpRequested{Purpose: service.purpose()}, false)
}

func (service *PasswordChangeService) finish(transaction string) {
	FlowUtils{}.Either(transaction != "", func() { service.send(transaction) }, func() {
		service.Error = "Confirm it is you before changing your password."
	})()
	service.Saving = false
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: service.purpose()}, false)
}

func (service *PasswordChangeService) problem(mismatched bool) string {
	switch {
	case service.NewPassword == "":
		return "Password is required."
	case mismatched:
		return "Passwords do not match."
	}
	return ""
}

func (service *PasswordChangeService) send(transaction string) {
	_, failure := service.authClient.ChangePassword(service.authorization, CredentialChange{StepUpTransactionID: transaction, Password: service.NewPassword})
	FlowUtils{}.Either(failure == nil, service.changed, func() {
		service.Error = HTTPUtils{}.Message(failure, "Unable to update password.", "Network error.")
	})()
}

func (service *PasswordChangeService) changed() {
	service.NewPassword = ""
	service.ConfirmPassword = ""
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Password updated."}, false)
}

func (service *PasswordChangeService) purpose() string { return "password" }
