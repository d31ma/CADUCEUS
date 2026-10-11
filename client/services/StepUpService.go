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
	address                   string
	Shown                     bool
	Password                  string
	Code                      string
	Error                     string
}

// @OnEvent({"replay":true})
func (service *StepUpService) HearSession(changed SessionChanged) {
	service.address = changed.Email
}

// @OnEvent
func (service *StepUpService) Prove(requested StepUpRequested) {
	code := strings.TrimSpace(service.Code)
	transaction := service.begin(service.ready(code))
	verified := service.verify(transaction, code)
	completed := StepUpCompleted{Purpose: requested.Purpose, Subject: requested.Subject, Transaction: service.finish(transaction, verified)}
	service.applicationEventPublisher.PublishEvent(completed, false)
}

func (service *StepUpService) ready(code string) bool {
	ready := !ChoiceUtils{}.Any(service.Password == "", code == "", service.address == "")
	FlowUtils{}.When(!ready, service.ask)()
	return ready
}

func (service *StepUpService) ask() {
	service.Error = "Confirm it is you: enter your password and one recovery code."
	service.Shown = true
}

func (service *StepUpService) begin(ready bool) string {
	if !ready {
		return ""
	}
	begun, _ := service.authClient.Begin(LoginStart{Address: service.address})
	service.Error = ChoiceUtils{}.Text(begun.TransactionID == "", "Unable to re-authenticate.", "")
	return begun.TransactionID
}

func (service *StepUpService) verify(transaction string, code string) bool {
	if transaction == "" {
		return false
	}
	_, _ = service.authClient.Password(PasswordFactor{TransactionID: transaction, Password: service.Password})
	verified, _ := service.authClient.Recovery(CodeFactor{TransactionID: transaction, Code: code})
	service.Error = ChoiceUtils{}.Text(verified.Satisfied, "", "That password and code did not confirm your identity.")
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
