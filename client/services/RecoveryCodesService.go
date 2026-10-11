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
func (service *RecoveryCodesService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
}

func (service *RecoveryCodesService) Generate() {
	service.Loading = true
	service.Error = ""
	service.Codes = []string{}
	service.applicationEventPublisher.PublishEvent(StepUpRequested{Purpose: service.purpose()}, false)
}

// @OnEvent
func (service *RecoveryCodesService) Proven(completed StepUpCompleted) {
	FlowUtils{}.When(completed.Purpose == service.purpose(), func() { service.finish(completed.Transaction) })()
}

func (service *RecoveryCodesService) Dismiss() {
	service.Codes = []string{}
	service.Error = ""
}

func (service *RecoveryCodesService) finish(transaction string) {
	service.Loading = false
	FlowUtils{}.Either(transaction == "", func() {
		service.Error = "Confirm it is you before replacing your recovery codes."
	}, func() { service.replace(transaction) })()
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: service.purpose()}, false)
}

func (service *RecoveryCodesService) replace(transaction string) {
	codes, failure := service.authClient.ReplaceRecoveryCodes(service.authorization, CredentialChange{StepUpTransactionID: transaction})
	FlowUtils{}.Either(failure == nil, func() { service.show(codes) }, func() {
		service.Error = HTTPUtils{}.Message(failure, "Unable to generate recovery codes.", "Network error.")
	})()
}

func (service *RecoveryCodesService) show(codes RecoveryCodeSet) {
	service.Codes = append([]string{}, codes.Codes...)
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Recovery codes replaced."}, false)
}

func (service *RecoveryCodesService) purpose() string { return "recovery-codes" }
