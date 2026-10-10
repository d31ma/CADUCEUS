// RecoveryCodesService replaces the account's one-time recovery codes after
// a fresh step-up. The plain codes stay in memory until they are dismissed.
// @Service
type RecoveryCodesService struct {
	// @Needs
	authClient                *AuthClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	Codes                     []string
	Loading                   bool
	Error                     string
}

// @OnStart
func (service *RecoveryCodesService) Start() {
	service.Codes = []string{}
}

// @OnEvent({"replay":true})
func (service *RecoveryCodesService) Open(session AccountSession) {
	service.authorization = "Bearer " + session.Token
}

func (service *RecoveryCodesService) Generate() {
	service.Loading = true
	service.Error = ""
	service.Codes = []string{}
	service.applicationEventPublisher.PublishEvent(StepUpRequested{Purpose: service.purpose()}, false)
}

// @OnEvent
func (service *RecoveryCodesService) Proven(completed StepUpCompleted) error {
	mine := completed.Purpose == service.purpose()
	FlowUtils{}.When(mine, func() { service.finish(completed.Transaction) })
	return nil
}

func (service *RecoveryCodesService) finish(transaction string) {
	service.Loading = false
	unproven := func() { service.Error = "Confirm it is you before replacing your recovery codes." }
	FlowUtils{}.Choose(transaction == "", unproven, func() { service.replace(transaction) })
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: service.purpose()}, false)
}

func (service *RecoveryCodesService) Dismiss() {
	service.Codes = []string{}
	service.Error = ""
}

func (service *RecoveryCodesService) replace(transaction string) {
	codes, failure := service.authClient.ReplaceRecoveryCodes(service.authorization, map[string]any{"stepUpTransactionId": transaction})
	FlowUtils{}.Choose(failure == nil, func() {
		service.Codes = append([]string{}, codes.Codes...)
		service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Recovery codes replaced."}, false)
	}, func() { service.Error = "Unable to generate recovery codes." })
}

func (service *RecoveryCodesService) purpose() string { return "recovery-codes" }
