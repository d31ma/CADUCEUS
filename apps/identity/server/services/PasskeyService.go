import (
	"mesail.dev/packages/login"
	"mesail.dev/packages/oidc"
)

// @Service
type PasskeyService struct {
	// @Needs
	grants *GrantService
	// @Needs
	failures *AccountFailureService
}

func (service *PasskeyService) Challenge() (PasskeyChallenge, error) {
	challenge, failed := login.SessionsFactory{}.Configured().PasskeyChallenge()
	return PasskeyChallenge{OptionsJson: challenge.OptionsJson, ChallengeKey: challenge.ChallengeKey}, service.failures.Failure(failed)
}

func (service *PasskeyService) SignIn(request PasskeySignIn) (Session, error) {
	grant, failed := login.SessionsFactory{}.Configured().PasskeySignIn(request.ChallengeKey, request.CredentialJson, request.DomainId, request.KeepSignedIn)
	return service.grants.Session(grant), service.failures.Failure(failed)
}

func (service *PasskeyService) List(authorization string) ([]Passkey, error) {
	passkeys, failed := login.SessionsFactory{}.Configured().Passkeys(authorization)
	return service.passkeys(passkeys), service.failures.Failure(failed)
}

func (service *PasskeyService) Setup(authorization string, request PasskeySetupRequest) (PasskeySetup, error) {
	setup, failed := login.SessionsFactory{}.Configured().SetupPasskey(authorization, request.CurrentPassword, request.OneTimeCode)
	return PasskeySetup{OptionsJson: setup.OptionsJson, SetupKey: setup.SetupKey}, service.failures.Failure(failed)
}

func (service *PasskeyService) Add(authorization string, request PasskeyConfirm) (Passkey, error) {
	added, failed := login.SessionsFactory{}.Configured().ConfirmPasskey(authorization, request.SetupKey, request.CredentialJson, request.Label)
	return service.passkey(added), service.failures.Failure(failed)
}

func (service *PasskeyService) Rename(authorization string, passkeyId string, request PasskeyRename) (Passkey, error) {
	renamed, failed := login.SessionsFactory{}.Configured().RenamePasskey(authorization, passkeyId, request.Label)
	return service.passkey(renamed), service.failures.Failure(failed)
}

func (service *PasskeyService) Remove(authorization string, passkeyId string, request PasskeyRemove) (Session, error) {
	removal := login.PasskeyRemoval{CurrentPassword: request.CurrentPassword, OneTimeCode: request.OneTimeCode, Kept: request.KeepSignedIn}
	grant, failed := login.SessionsFactory{}.Configured().RemovePasskey(authorization, passkeyId, removal)
	return service.grants.Session(grant), service.failures.Failure(failed)
}

func (service *PasskeyService) passkeys(list []oidc.Passkey) []Passkey {
	passkeys := []Passkey{}
	for _, item := range list {
		passkeys = append(passkeys, service.passkey(item))
	}
	return passkeys
}

func (service *PasskeyService) passkey(item oidc.Passkey) Passkey {
	return Passkey{PasskeyId: item.PasskeyId, Label: item.Label, CreatedAt: item.CreatedAt, LastUsedAt: item.LastUsedAt}
}
