import "mesail.dev/packages/login"

// @Service
type SessionService struct {
	// @Needs
	grants *GrantService
	// @Needs
	failures *AccountFailureService
}

func (service *SessionService) SignIn(request SignInRequest) (Session, error) {
	grant, failed := login.SessionsFactory{}.Configured().SignIn(request.Login, request.Password, request.OneTimeCode, request.KeepSignedIn)
	return service.grants.Session(grant), service.failures.Failure(failed)
}

func (service *SessionService) Refresh(request RefreshRequest) (Session, error) {
	grant, failed := login.SessionsFactory{}.Configured().Refresh(request.SessionKey)
	return service.grants.Session(grant), service.failures.Failure(failed)
}

func (service *SessionService) Switch(request DomainSwitch) (Session, error) {
	grant, failed := login.SessionsFactory{}.Configured().Switch(request.SessionKey, request.DomainId)
	return service.grants.Session(grant), service.failures.Failure(failed)
}

func (service *SessionService) SignOut(request RefreshRequest) error {
	return service.failures.Failure(login.SessionsFactory{}.Configured().SignOut(request.SessionKey))
}

func (service *SessionService) SignOutEverywhere(authorization string) error {
	return service.failures.Failure(login.SessionsFactory{}.Configured().SignOutEverywhere(authorization))
}

func (service *SessionService) ChangePassword(authorization string, request PasswordChange) (Session, error) {
	change := login.PasswordChange{CurrentPassword: request.CurrentPassword, NewPassword: request.NewPassword, OneTimeCode: request.OneTimeCode, Kept: request.KeepSignedIn}
	grant, failed := login.SessionsFactory{}.Configured().ChangePassword(authorization, change)
	return service.grants.Session(grant), service.failures.Failure(failed)
}
