import (
	"cmp"
	"regexp"
	"slices"
	"strings"
)

// PasskeysService lists the account's sign-in devices and adds or removes
// passkeys after a fresh step-up. PasskeyCeremonyService.js runs the
// browser's WebAuthn ceremony.
// @Service
type PasskeysService struct {
	// @Needs
	authClient                *AuthClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	address                   string
	timeZoneOffset            float64
	Devices                   []SignInDevice
	Adding                    bool
	Provision                 *PasskeyProvision
	NewName                   string
	Error                     string
	Loading                   bool
}

// @OnEvent({"replay":true})
func (service *PasskeysService) Open(session AccountSession) error {
	service.authorization = "Bearer " + session.Token
	service.address = session.Email
	service.refresh()
	service.announce()
	return nil
}

// @OnEvent({"replay":true})
func (service *PasskeysService) Learn(platform DevicePlatform) {
	service.timeZoneOffset = platform.TimeZoneOffset
}

func (service *PasskeysService) PasskeyRows() []DeviceRow {
	return service.rows(slices.DeleteFunc(slices.Clone(service.Devices), func(device SignInDevice) bool {
		return !FlowUtils{}.Either(device.Type == "webauthn", device.CredentialID != "")
	}))
}

func (service *PasskeysService) AuthenticatorRows() []DeviceRow {
	return service.rows(slices.DeleteFunc(slices.Clone(service.Devices), func(device SignInDevice) bool { return device.Type != "totp" }))
}

// StartAdd opens the passkey wizard after a fresh step-up, or closes it.
func (service *PasskeysService) StartAdd() {
	FlowUtils{}.Choose(service.Adding, service.closeWizard, func() {
		service.Adding = true
		service.Provision = nil
		service.applicationEventPublisher.PublishEvent(StepUpRequested{Purpose: service.adding()}, false)
	})
}

// Confirm asks the browser to create the passkey for the open provision.
func (service *PasskeysService) Confirm() {
	provision := service.provision()
	ready := FlowUtils{}.Both(provision.Challenge != "", provision.RelyingPartyID != "")
	FlowUtils{}.Choose(ready, func() { service.ceremony(provision) }, func() { service.Error = "Passkey setup has expired. Start again." })
}

// @OnEvent
func (service *PasskeysService) Finish(finished PasskeyCeremonyFinished) error {
	FlowUtils{}.Choose(finished.Problem == "", func() { service.register(finished) }, func() { service.Error = finished.Problem })
	service.Loading = false
	service.announce()
	return nil
}

// @OnEvent
func (service *PasskeysService) Remove(accepted ConfirmationAccepted) {
	FlowUtils{}.When(accepted.Topic == service.removing(), func() {
		service.applicationEventPublisher.PublishEvent(StepUpRequested{Purpose: service.removing(), Subject: accepted.Subject}, false)
	})
}

// @OnEvent
func (service *PasskeysService) Proven(completed StepUpCompleted) error {
	handlers := map[string]func(StepUpCompleted){service.adding(): service.begin, service.removing(): service.remove}
	handler, known := handlers[completed.Purpose]
	FlowUtils{}.When(known, func() { handler(completed) })
	service.announce()
	return nil
}

// @OnEvent
func (service *PasskeysService) Changed(changed SignInDevicesChanged) error {
	service.refresh()
	service.announce()
	return nil
}

func (service *PasskeysService) refresh() {
	list, _ := service.authClient.Passkeys(service.authorization)
	service.Devices = append([]SignInDevice{}, list.Passkeys...)
}

func (service *PasskeysService) closeWizard() {
	service.Adding = false
	service.Provision = nil
	service.NewName = ""
	service.Error = ""
}

func (service *PasskeysService) begin(completed StepUpCompleted) {
	provision, failure := service.authClient.BeginPasskey(service.authorization, service.stepUpBody(completed.Transaction))
	started := FlowUtils{}.Both(completed.Transaction != "", failure == nil)
	FlowUtils{}.Choose(started, func() { service.Provision = &provision }, func() {
		FlowUtils{}.When(completed.Transaction != "", func() { service.toast("Failed to start device setup.") })
		service.Adding = false
	})
}

func (service *PasskeysService) ceremony(provision PasskeyProvision) {
	service.Loading = true
	service.Error = ""
	user := regexp.MustCompile(`[^A-Za-z0-9_-]`).ReplaceAllString(cmp.Or(provision.PrincipalID, service.address), "")
	address := cmp.Or(service.address, "mailbox")
	service.applicationEventPublisher.PublishEvent(PasskeyCeremonyRequested{
		Challenge: provision.Challenge, RelyingPartyID: provision.RelyingPartyID, UserID: user,
		Name: address, DisplayName: cmp.Or(strings.TrimSpace(service.NewName), address),
	}, false)
}

func (service *PasskeysService) register(finished PasskeyCeremonyFinished) {
	_, failure := service.authClient.FinishPasskey(service.authorization, map[string]any{"attestationObject": finished.AttestationObject, "clientDataJSON": finished.ClientDataJSON})
	FlowUtils{}.Choose(failure == nil, func() {
		service.toast("Passkey added.")
		service.closeWizard()
		service.refresh()
	}, func() { service.Error = "Unable to save passkey." })
}

func (service *PasskeysService) remove(completed StepUpCompleted) {
	FlowUtils{}.When(completed.Transaction != "", func() {
		body := service.stepUpBody(completed.Transaction)
		body["credentialId"] = completed.Subject
		_, failure := service.authClient.RemovePasskey(service.authorization, body)
		FlowUtils{}.When(failure == nil, service.refresh)
		service.toast(FlowUtils{}.Text(failure == nil, "Passkey removed.", "Unable to remove that passkey."))
	})
}

func (service *PasskeysService) provision() PasskeyProvision {
	if service.Provision == nil {
		return PasskeyProvision{}
	}
	return *service.Provision
}

func (service *PasskeysService) rows(devices []SignInDevice) []DeviceRow {
	rows := []DeviceRow{}
	for _, device := range devices {
		rows = append(rows, DeviceRow{ID: cmp.Or(device.ID, device.CredentialID), Name: device.Name, CreatedAtLabel: DateLabelUtils{}.Date(device.CreatedAt, service.timeZoneOffset)})
	}
	return rows
}

func (service *PasskeysService) stepUpBody(transaction string) map[string]any {
	return map[string]any{"stepUpTransactionId": transaction}
}

func (service *PasskeysService) adding() string { return "passkey-add" }

func (service *PasskeysService) removing() string { return "passkey-remove" }

func (service *PasskeysService) toast(message string) {
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (service *PasskeysService) announce() {
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "passkeys"}, false)
}
