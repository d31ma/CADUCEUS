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
func (service *PasswordChangeService) Open(session AccountSession) {
	service.authorization = "Bearer " + session.Token
}

func (service *PasswordChangeService) Update() {
	problem := service.problem(FlowUtils{}.Both(service.ConfirmPassword != "", service.ConfirmPassword != service.NewPassword))
	service.Error = problem
	FlowUtils{}.When(problem == "", func() {
		service.Saving = true
		service.applicationEventPublisher.PublishEvent(StepUpRequested{Purpose: service.purpose()}, false)
	})
}

// @OnEvent
func (service *PasswordChangeService) Proven(completed StepUpCompleted) error {
	FlowUtils{}.When(completed.Purpose == service.purpose(), func() {
		FlowUtils{}.Choose(completed.Transaction != "", func() { service.send(completed.Transaction) }, func() {
			service.Error = "Confirm it is you before changing your password."
		})
		service.Saving = false
		service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: service.purpose()}, false)
	})
	return nil
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
	_, failure := service.authClient.SetPassword(service.authorization, map[string]any{"stepUpTransactionId": transaction, "password": service.NewPassword})
	FlowUtils{}.Choose(failure == nil, func() {
		service.NewPassword = ""
		service.ConfirmPassword = ""
		service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Password updated."}, false)
	}, func() { service.Error = "Unable to update password." })
}

func (service *PasswordChangeService) purpose() string { return "password" }
