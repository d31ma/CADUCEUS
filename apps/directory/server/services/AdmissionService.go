import (
	"cmp"

	"mesail.dev/packages/oidc"
)

// @Service
type AdmissionService struct {
	// @Needs
	callers *CallerService
	// @Needs
	members *MemberService
}

func (service *AdmissionService) Admit(authorization string) Admission {
	caller, refused := service.callers.Caller(authorization)
	return Admission{Caller: caller, Refusal: cmp.Or(refused, service.rememberUnless(refused, caller))}
}

func (service *AdmissionService) rememberUnless(refused error, caller oidc.Caller) error {
	if refused != nil {
		return refused
	}
	_, failure := service.members.Remember(caller)
	return failure
}
