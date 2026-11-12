import "mesail.dev/packages/oidc"

// @Service
type CallerService struct{}

func (callers *CallerService) Caller(authorization string) (oidc.Caller, error) {
	caller, failure := oidc.Configured().Identify(authorization)
	return caller, callers.refusal(failure)
}

func (callers *CallerService) Operator(authorization string) error {
	caller, failure := callers.Caller(authorization)
	return callers.operatorOnly(failure, caller)
}

func (callers *CallerService) IsInstallWide(domainId string) bool {
	return oidc.InstallWide{}.Matches(domainId)
}

func (callers *CallerService) refusal(failure error) error {
	if failure != nil {
		return &Unauthenticated{}
	}
	return nil
}

func (callers *CallerService) operatorOnly(failure error, caller oidc.Caller) error {
	if failure == nil && !callers.IsInstallWide(caller.DomainId) {
		return &NotFound{What: "install-wide route"}
	}
	return failure
}
