import (
	"cmp"
	"slices"

	"mesail.dev/packages/oidc"
)

// @Service
type CallPushesService struct {
	// @Needs
	callers *CallerService
	// @Needs
	members *MembershipService
	// @Needs
	pushes *ContentFreePushService
}

func (service *CallPushesService) Request(authorization string, request CallPush) (CallPushReport, error) {
	caller, failure := service.callers.Caller(authorization)
	failure = cmp.Or(failure, service.serviceOnly(caller), service.oneDomain(caller), service.kindCheck(request.Kind), service.peopleCheck(request.ToPersonIds))
	membership, failure := service.membershipUnless(failure, caller.DomainId, request.ToPersonIds)
	push := ContentFreePush{DomainId: caller.DomainId, PersonIds: membership.Active, Kind: service.kinds()[request.Kind]}
	report, failure := service.pushUnless(failure, push)
	return service.withOutsiders(report, membership.Outsiders), failure
}

func (service *CallPushesService) serviceOnly(caller oidc.Caller) error {
	if !slices.Contains(caller.Roles, service.serviceRole()) {
		return NotService{}.ServiceOnly()
	}
	return nil
}

func (service *CallPushesService) oneDomain(caller oidc.Caller) error {
	if service.callers.IsInstallWide(caller.DomainId) {
		return NotFound{}.Domain()
	}
	return nil
}

func (service *CallPushesService) kindCheck(kind string) error {
	if _, found := service.kinds()[kind]; !found {
		return InvalidRequest{}.BadCallKind()
	}
	return nil
}

func (service *CallPushesService) peopleCheck(personIds []string) error {
	if len(personIds) < 1 || len(personIds) > service.mostPeople() || slices.ContainsFunc(personIds, service.badId) {
		return InvalidRequest{}.BadPersonList()
	}
	return nil
}

func (service *CallPushesService) badId(personId string) bool {
	return personId == "" || len(personId) > service.longestId()
}

func (service *CallPushesService) membershipUnless(failure error, domainId string, personIds []string) (Membership, error) {
	if failure != nil {
		return Membership{}, failure
	}
	return service.members.Split(domainId, personIds)
}

func (service *CallPushesService) pushUnless(failure error, push ContentFreePush) (CallPushReport, error) {
	if failure != nil {
		return CallPushReport{}, failure
	}
	return service.pushes.Push(push)
}

func (service *CallPushesService) withOutsiders(report CallPushReport, outsiders []string) CallPushReport {
	return CallPushReport{Pushed: report.Pushed, Skipped: report.Skipped + len(outsiders)}
}

func (service *CallPushesService) kinds() map[string]string {
	return map[string]string{
		"incoming": service.pushes.IncomingCall(),
		"missed":   service.pushes.MissedCall(),
	}
}

func (service *CallPushesService) serviceRole() string {
	return "Service"
}

func (service *CallPushesService) mostPeople() int {
	return 49
}

func (service *CallPushesService) longestId() int {
	return 64
}
