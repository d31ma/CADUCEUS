import "regexp"

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
	service.Name = FlowUtils{}.Text(service.Name == "", "Authenticator app", service.Name)
}

// @OnEvent({"replay":true})
func (service *AuthenticatorAppService) Open(session AccountSession) {
	service.authorization = "Bearer " + session.Token
}

// Toggle opens enrollment after a fresh step-up, or cancels an open one.
func (service *AuthenticatorAppService) Toggle() {
	FlowUtils{}.Choose(service.Enrollment != nil, service.cancel, func() {
		service.Loading = true
		service.Error = ""
		service.applicationEventPublisher.PublishEvent(StepUpRequested{Purpose: service.purpose()}, false)
	})
}

// @OnEvent
func (service *AuthenticatorAppService) Proven(completed StepUpCompleted) error {
	FlowUtils{}.When(completed.Purpose == service.purpose(), func() { service.enroll(completed.Transaction) })
	return nil
}

func (service *AuthenticatorAppService) Confirm() error {
	valid := FlowUtils{}.Both(service.secret() != "", regexp.MustCompile(`^\d{6}$`).MatchString(service.Code))
	FlowUtils{}.Choose(valid, service.activate, func() { service.Error = "Enter the current 6-digit authenticator code." })
	return nil
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
	return service.authClient.EnrollAuthenticator(service.authorization, map[string]any{"stepUpTransactionId": transaction})
}

func (service *AuthenticatorAppService) showEnrollment(enrollment AuthenticatorEnrollment, failure error, transaction string) {
	switch {
	case transaction == "":
		service.Error = "Confirm it is you before enrolling an authenticator."
	case failure != nil:
		service.Error = "Unable to start authenticator enrollment."
	default:
		service.Enrollment = &enrollment
	}
}

func (service *AuthenticatorAppService) activate() {
	service.Loading = true
	service.Error = ""
	_, failure := service.authClient.ActivateAuthenticator(service.authorization, map[string]any{"code": service.Code})
	service.Loading = false
	FlowUtils{}.Choose(failure == nil, func() {
		service.cancel()
		service.applicationEventPublisher.PublishEvent(SignInDevicesChanged{Reason: service.purpose()}, false)
		service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Authenticator app enabled."}, false)
	}, func() { service.Error = "Unable to verify authenticator code." })
}

func (service *AuthenticatorAppService) secret() string {
	if service.Enrollment == nil {
		return ""
	}
	return service.Enrollment.Secret
}

func (service *AuthenticatorAppService) purpose() string { return "authenticator" }
