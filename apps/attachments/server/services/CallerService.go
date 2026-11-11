import "mesail.dev/packages/oidc"

// @Service
type CallerService struct {
}

func (service *CallerService) Caller(authorization string) (oidc.Caller, error) {
	caller, failure := oidc.Configured().Identify(authorization)
	return caller, service.refusal(failure)
}

func (service *CallerService) refusal(failure error) error {
	if failure != nil {
		return &Unauthenticated{}
	}
	return nil
}
