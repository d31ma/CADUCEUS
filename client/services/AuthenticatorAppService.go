import (
	"cmp"
	"regexp"
)

// AuthenticatorAppService enrolls a time-based authenticator app as the
// secondary verification factor, after a fresh step-up.
// @Service
type AuthenticatorAppService struct {
	// @Needs
	authClient                *AuthClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	Enrollment                *AuthenticatorEnrollment
	Code                      string
	// @SessionState
	Name    string
	Loading bool
	Error   string
}

// @OnStart
func (service *AuthenticatorAppService) Start() {
	service.Name = cmp.Or(service.Name, "Authenticator app")
}

// @OnEvent({"replay":true})
func (service *AuthenticatorAppService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
}

// Toggle opens enrollment after a fresh step-up, or cancels an open one.
func (service *AuthenticatorAppService) Toggle() {
	FlowUtils{}.Either(service.Enrollment != nil, service.cancel, service.prove)()
}

// @OnEvent
func (service *AuthenticatorAppService) Proven(completed StepUpCompleted) {
	FlowUtils{}.When(completed.Purpose == service.purpose(), func() { service.enroll(completed.Transaction) })()
}

func (service *AuthenticatorAppService) Confirm() error {
	valid := ChoiceUtils{}.All(service.secret() != "", regexp.MustCompile(`^\d{6}$`).MatchString(service.Code))
	FlowUtils{}.Either(valid, service.activate, func() { service.Error = "Enter the current 6-digit authenticator code." })()
	return nil
}

func (service *AuthenticatorAppService) prove() {
	service.Loading = true
	service.Error = ""
	service.applicationEventPublisher.PublishEvent(StepUpRequested{Purpose: service.purpose()}, false)
}

func (service *AuthenticatorAppService) cancel() {
	service.Enrollment = nil
	service.Code = ""
	service.Error = ""
}

func (service *AuthenticatorAppService) enroll(transaction string) {
	enrollment, failure := service.request(transaction)
	service.Loading = false
	service.showEnrollment(enrollment, failure, transaction)
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: service.purpose()}, false)
}

func (service *AuthenticatorAppService) request(transaction string) (AuthenticatorEnrollment, error) {
	if transaction == "" {
		return AuthenticatorEnrollment{}, nil
	}
	return service.authClient.EnrollAuthenticator(service.authorization, CredentialChange{StepUpTransactionID: transaction})
}

func (service *AuthenticatorAppService) showEnrollment(enrollment AuthenticatorEnrollment, failure error, transaction string) {
	switch {
	case transaction == "":
		service.Error = "Confirm it is you before enrolling an authenticator."
	case failure != nil:
		service.Error = HTTPUtils{}.Message(failure, "Unable to start authenticator enrollment.", service.network())
	default:
		service.Enrollment = &enrollment
	}
}

func (service *AuthenticatorAppService) activate() {
	service.Loading = true
	service.Error = ""
	_, failure := service.authClient.ActivateAuthenticator(service.authorization, map[string]any{"code": service.Code})
	service.Loading = false
	FlowUtils{}.Either(failure == nil, service.activated, func() {
		service.Error = HTTPUtils{}.Message(failure, "Unable to verify authenticator code.", service.network())
	})()
}

func (service *AuthenticatorAppService) activated() {
	service.cancel()
	service.applicationEventPublisher.PublishEvent(SignInDevicesChanged{Reason: service.purpose()}, false)
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Authenticator app enabled."}, false)
}

func (service *AuthenticatorAppService) secret() string {
	if service.Enrollment == nil {
		return ""
	}
	return service.Enrollment.Secret
}

func (service *AuthenticatorAppService) purpose() string { return "authenticator" }

func (service *AuthenticatorAppService) network() string { return "Network error." }
