import (
	"cmp"
	"errors"

	"mesail.dev/packages/login"
	"mesail.dev/packages/oidc"
)

// @Service
type AccountFailureService struct {
}

func (service *AccountFailureService) Failure(failed error) error {
	return cmp.Or(service.policy(failed), service.known(failed), service.refusal(failed))
}

func (service *AccountFailureService) policy(failed error) error {
	refused := oidc.AdminRefused{}
	if errors.As(failed, &refused) {
		return BadRequest{}.PasswordPolicy(refused.Message)
	}
	return nil
}

func (service *AccountFailureService) known(failed error) error {
	var answer error
	for _, known := range service.cases() {
		answer = cmp.Or(answer, service.matching(failed, known))
	}
	return answer
}

func (service *AccountFailureService) matching(failed error, known FailureCase) error {
	if errors.Is(failed, known.Cause) {
		return known.Answer
	}
	return nil
}

func (service *AccountFailureService) refusal(failed error) error {
	if failed == nil {
		return nil
	}
	return Unauthenticated{}.SignInFailed()
}

func (service *AccountFailureService) cases() []FailureCase {
	return []FailureCase{
		{Cause: oidc.InvalidCode{}, Answer: BadRequest{}.InvalidCode()},
		{Cause: oidc.SetupExpired{}, Answer: BadRequest{}.SetupExpired()},
		{Cause: oidc.InvalidPasskey{}, Answer: BadRequest{}.InvalidPasskey()},
		{Cause: oidc.TooManyPasskeys{}, Answer: Conflict{}.TooManyPasskeys()},
		{Cause: oidc.LabelExists{}, Answer: Conflict{}.LabelExists()},
		{Cause: oidc.InvalidLabel{}, Answer: BadRequest{}.InvalidLabel()},
		{Cause: oidc.PasskeyNotFound{}, Answer: NotFound{}.Passkey()},
		{Cause: oidc.AccountLocked{}, Answer: RateLimited{}.Locked()},
		{Cause: oidc.SessionUnavailable{}, Answer: Unavailable{}.SignIn()},
		{Cause: login.NotAMember{}, Answer: Forbidden{}.NotAMember()},
		{Cause: login.NoDomain{}, Answer: Forbidden{}.NoDomain()},
		{Cause: login.AccessRevoked{}, Answer: Forbidden{}.AccessRevoked()},
		{Cause: login.NotFound{}, Answer: NotFound{}.Domain()},
	}
}
