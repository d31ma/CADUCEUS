import "strings"

// StepUpService proves a factor again before a credential changes: on
// StepUpRequested it runs a fresh password and recovery-code sign-in and
// answers StepUpCompleted with the satisfied transaction, or none. Changing
// a credential is what an account takeover needs, so a session alone never
// suffices.
// @Service
type StepUpService struct {
	// @Needs
	authClient                *AuthClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	address       string
	Shown         bool
	Password      string
	Code          string
	Error         string
}

// @OnEvent({"replay":true})
func (service *StepUpService) Open(session AccountSession) {
	service.authorization = "Bearer " + session.Token
	service.address = session.Email
}

// @OnEvent
func (service *StepUpService) Prove(requested StepUpRequested) error {
	transaction := service.begin(service.ready(strings.TrimSpace(service.Code), service.Password))
	verified := service.verify(transaction, strings.TrimSpace(service.Code), service.Password)
	completed := StepUpCompleted{Purpose: requested.Purpose, Subject: requested.Subject, Transaction: service.finish(transaction, verified)}
	service.applicationEventPublisher.PublishEvent(completed, false)
	return nil
}

func (service *StepUpService) ready(code string, password string) bool {
	if password == "" || code == "" || service.address == "" {
		service.Error = "Confirm it is you: enter your password and one recovery code."
		service.Shown = true
		return false
	}
	return true
}

func (service *StepUpService) begin(ready bool) string {
	if !ready {
		return ""
	}
	begun, _ := service.authClient.BeginLogin(service.authorization, map[string]any{"address": service.address})
	service.Error = FlowUtils{}.Text(begun.TransactionID == "", "Unable to re-authenticate.", "")
	return begun.TransactionID
}

func (service *StepUpService) verify(transaction string, code string, password string) bool {
	if transaction == "" {
		return false
	}
	_, _ = service.authClient.LoginPassword(service.authorization, map[string]any{service.transactionKey(): transaction, "password": password})
	verified, _ := service.authClient.LoginRecovery(service.authorization, map[string]any{service.transactionKey(): transaction, "code": code})
	service.Error = FlowUtils{}.Text(verified.Satisfied, "", "That password and code did not confirm your identity.")
	service.Shown = !verified.Satisfied
	return verified.Satisfied
}

func (service *StepUpService) finish(transaction string, verified bool) string {
	if !verified {
		return ""
	}
	service.Code = ""
	service.Password = ""
	service.Error = ""
	service.Shown = false
	return transaction
}

func (service *StepUpService) transactionKey() string { return "transactionId" }
