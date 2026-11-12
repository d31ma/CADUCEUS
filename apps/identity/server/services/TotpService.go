import "mesail.dev/packages/login"

// @Service
type TotpService struct {
	// @Needs
	failures *AccountFailureService
}

func (service *TotpService) Status(authorization string) (TotpStatus, error) {
	enabled, failed := login.SessionsFactory{}.Configured().TotpEnabled(authorization)
	return TotpStatus{Enabled: enabled}, service.failures.Failure(failed)
}

func (service *TotpService) Setup(authorization string, request TotpSetupRequest) (TotpSetup, error) {
	setup, failed := login.SessionsFactory{}.Configured().SetupTotp(authorization, request.CurrentPassword)
	return TotpSetup{Secret: setup.Secret, OtpauthUri: setup.Uri, QrImage: setup.QrImage, SetupKey: setup.SetupKey}, service.failures.Failure(failed)
}

func (service *TotpService) Confirm(authorization string, request TotpConfirm) error {
	return service.failures.Failure(login.SessionsFactory{}.Configured().ConfirmTotp(authorization, request.SetupKey, request.Code))
}

func (service *TotpService) Remove(authorization string, request TotpRemove) error {
	return service.failures.Failure(login.SessionsFactory{}.Configured().RemoveTotp(authorization, request.CurrentPassword, request.OneTimeCode))
}
