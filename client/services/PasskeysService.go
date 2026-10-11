import (
	"cmp"
	"slices"
	"strings"
)

// PasskeysService lists the account's sign-in devices and adds or removes
// passkeys after a fresh step-up. PasskeyService.js runs the browser's
// WebAuthn ceremony for PasskeyRequested and answers PasskeyAnswered.
// @Service
type PasskeysService struct {
	// @Needs
	authClient                *AuthClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	address                   string
	supported                 bool
	timeZoneOffset            float64
	Devices                   []SignInDevice
	Adding                    bool
	Provision                 *PasskeyEnrollment
	NewName                   string
	Error                     string
	Loading                   bool
}

// @OnStart
func (service *PasskeysService) Start() {
	service.Devices = []SignInDevice{}
}

// @OnEvent({"replay":true})
func (service *PasskeysService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
	service.address = changed.Email
	refused := false
	FlowUtils{}.When(changed.SignedIn, func() { refused = service.refresh() })()
	service.announce(refused)
}

// @OnEvent({"replay":true})
func (service *PasskeysService) HearFeatures(features BrowserFeatures) {
	service.supported = features.Passkeys
}

// @OnEvent({"replay":true})
func (service *PasskeysService) Learn(platform DevicePlatform) {
	service.timeZoneOffset = platform.TimeZoneOffset
}

func (service *PasskeysService) PasskeyRows() []DeviceRow {
	return service.rows(slices.DeleteFunc(slices.Clone(service.Devices), func(device SignInDevice) bool {
		return !ChoiceUtils{}.Any(device.Type == "webauthn", device.CredentialID != "")
	}))
}

func (service *PasskeysService) AuthenticatorRows() []DeviceRow {
	return service.rows(slices.DeleteFunc(slices.Clone(service.Devices), func(device SignInDevice) bool { return device.Type != "totp" }))
}

// StartAdd opens the passkey wizard after a fresh step-up, or closes it.
func (service *PasskeysService) StartAdd() {
	FlowUtils{}.Either(service.Adding, service.closeWizard, service.prove)()
}

// Confirm asks the browser to create the passkey for the open provision.
func (service *PasskeysService) Confirm() {
	provision := *cmp.Or(service.Provision, &PasskeyEnrollment{})
	ready := ChoiceUtils{}.All(provision.Challenge != "", provision.RelyingPartyID != "")
	problem := service.ceremonyProblem(ready)
	FlowUtils{}.Either(problem == "", func() { service.ceremony(provision) }, func() { service.Error = problem })()
}

// @OnEvent
func (service *PasskeysService) Finish(answered PasskeyAnswered) {
	FlowUtils{}.When(answered.Mode == service.creation(), func() { service.finish(answered) })()
}

// @OnEvent
func (service *PasskeysService) Remove(given ConfirmationGiven) {
	FlowUtils{}.When(given.Action == service.removing(), func() {
		service.applicationEventPublisher.PublishEvent(StepUpRequested{Purpose: service.removing(), Subject: given.ID}, false)
	})()
}

// @OnEvent
func (service *PasskeysService) Proven(completed StepUpCompleted) {
	handlers := map[string]func(StepUpCompleted){service.adding(): service.begin, service.removing(): service.remove}
	handler, known := handlers[completed.Purpose]
	FlowUtils{}.When(known, func() { handler(completed) })()
	service.announce(false)
}

// @OnEvent
func (service *PasskeysService) Changed(_ SignInDevicesChanged) {
	service.announce(service.refresh())
}

// refresh reads the sign-in devices and says whether the server refused the read.
func (service *PasskeysService) refresh() bool {
	list, failure := service.authClient.Passkeys(service.authorization)
	FlowUtils{}.When(failure == nil, func() { service.Devices = append([]SignInDevice{}, list.Passkeys...) })()
	return HTTPUtils{}.Refused(failure)
}

func (service *PasskeysService) prove() {
	service.Adding = true
	service.Provision = nil
	service.applicationEventPublisher.PublishEvent(StepUpRequested{Purpose: service.adding()}, false)
}

func (service *PasskeysService) closeWizard() {
	service.Adding = false
	service.Provision = nil
	service.NewName = ""
	service.Error = ""
}

func (service *PasskeysService) begin(completed StepUpCompleted) {
	FlowUtils{}.Either(completed.Transaction != "", func() { service.provision(completed.Transaction) }, func() { service.Adding = false })()
}

func (service *PasskeysService) provision(transaction string) {
	provision, failure := service.authClient.BeginPasskeyEnrollment(service.authorization, CredentialChange{StepUpTransactionID: transaction})
	FlowUtils{}.Either(failure == nil, func() { service.Provision = &provision }, func() {
		service.toast(HTTPUtils{}.Message(failure, "Failed to start device setup.", service.network()))
		service.Adding = false
	})()
}

func (service *PasskeysService) ceremonyProblem(ready bool) string {
	switch {
	case !ready:
		return "Passkey setup has expired. Start again."
	case !service.supported:
		return "Passkeys are not supported on this device."
	}
	return ""
}

func (service *PasskeysService) ceremony(provision PasskeyEnrollment) {
	service.Loading = true
	service.Error = ""
	address := cmp.Or(service.address, "mailbox")
	service.applicationEventPublisher.PublishEvent(PasskeyRequested{
		Mode: service.creation(), Challenge: provision.Challenge, RelyingPartyID: provision.RelyingPartyID, Credentials: []string{},
		UserID: cmp.Or(provision.PrincipalID, address), UserName: address, DisplayName: cmp.Or(strings.TrimSpace(service.NewName), address),
	}, false)
}

func (service *PasskeysService) finish(answered PasskeyAnswered) {
	FlowUtils{}.Either(answered.Failure == "", func() { service.register(answered) }, func() {
		service.Error = ChoiceUtils{}.Text(answered.Failure == "cancelled", "Passkey creation was cancelled.", "Unable to add passkey.")
	})()
	service.Loading = false
	service.announce(false)
}

func (service *PasskeysService) register(answered PasskeyAnswered) {
	attestation := PasskeyAttestation{AttestationObject: answered.AttestationObject, ClientDataJSON: answered.ClientDataJSON}
	_, failure := service.authClient.FinishPasskeyEnrollment(service.authorization, attestation)
	FlowUtils{}.Either(failure == nil, service.registered, func() {
		service.Error = HTTPUtils{}.Message(failure, "Unable to save passkey.", service.network())
	})()
}

func (service *PasskeysService) registered() {
	service.toast("Passkey added.")
	service.closeWizard()
	service.refresh()
}

func (service *PasskeysService) remove(completed StepUpCompleted) {
	FlowUtils{}.When(completed.Transaction != "", func() { service.forget(completed.Transaction, completed.Subject) })()
}

func (service *PasskeysService) forget(transaction string, credential string) {
	_, failure := service.authClient.RemovePasskey(service.authorization, map[string]any{"stepUpTransactionId": transaction, "credentialId": credential})
	FlowUtils{}.When(failure == nil, func() { service.refresh() })()
	service.toast(ChoiceUtils{}.Text(failure == nil, "Passkey removed.", HTTPUtils{}.Message(failure, "Unable to remove that passkey.", service.network())))
}

func (service *PasskeysService) rows(devices []SignInDevice) []DeviceRow {
	rows := []DeviceRow{}
	for _, device := range devices {
		rows = append(rows, DeviceRow{ID: cmp.Or(device.ID, device.CredentialID), Name: device.Name, CreatedAtLabel: TimestampLabelUtils{}.Date(device.CreatedAt, service.timeZoneOffset)})
	}
	return rows
}

func (service *PasskeysService) adding() string { return "passkey-add" }

func (service *PasskeysService) removing() string { return "passkey-remove" }

func (service *PasskeysService) creation() string { return "create" }

func (service *PasskeysService) network() string { return "Network error." }

func (service *PasskeysService) toast(message string) {
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (service *PasskeysService) announce(refused bool) {
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "passkeys", Refused: refused}, false)
}
