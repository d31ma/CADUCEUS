import (
	"cmp"
	"net/http"

	"mesail.dev/packages/oidc"
)

// @Service
type PersonRouteService struct {
	// @Needs
	callers *CallerService
	// @Needs
	people *PersonService
	// @Needs
	seeds *PersonSeedService
	// @Needs
	devices *DeviceService
}

func (service *PersonRouteService) Search(request *http.Request) (PersonPage, error) {
	caller, refused := service.callers.Caller(request.Header.Get(service.authorizationHeader()))
	query, invalid := PageWindow{}.Query(request.URL.Query())
	return service.searchUnless(cmp.Or(refused, service.callers.Operator(caller), invalid), query)
}

func (service *PersonRouteService) Record(authorization string, personId string) (PersonRecord, error) {
	caller, refused := service.callers.Caller(authorization)
	return service.recordUnless(cmp.Or(refused, service.callers.InstallWide(caller)), personId)
}

func (service *PersonRouteService) Seed(authorization string, seed PersonSeed) (PersonRecord, error) {
	caller, refused := service.callers.Caller(authorization)
	return service.seedUnless(cmp.Or(refused, service.callers.InstallWide(caller)), seed)
}

func (service *PersonRouteService) Change(authorization string, personId string, change PersonChange) (PersonSummary, error) {
	caller, refused := service.callers.Caller(authorization)
	summary, failure := service.changeUnless(refused, caller, personId, change)
	return summary, Steps{}.Unless(failure, func() error { return service.devicesAfter(summary) })
}

func (service *PersonRouteService) ResetPassword(authorization string, personId string, reset PasswordReset) error {
	caller, refused := service.callers.Caller(authorization)
	return service.resetUnless(refused, caller, personId, reset)
}

func (service *PersonRouteService) RemoveTotp(authorization string, personId string) error {
	caller, refused := service.callers.Caller(authorization)
	return service.removeTotpUnless(refused, caller, personId)
}

func (service *PersonRouteService) RemovePasskeys(authorization string, personId string) error {
	caller, refused := service.callers.Caller(authorization)
	return service.removePasskeysUnless(refused, caller, personId)
}

func (service *PersonRouteService) EndSessions(authorization string, personId string) error {
	caller, refused := service.callers.Caller(authorization)
	return service.endSessionsUnless(cmp.Or(refused, service.callers.InstallWide(caller)), personId)
}

func (service *PersonRouteService) Delete(authorization string, personId string) error {
	caller, refused := service.callers.Caller(authorization)
	failure := service.deleteUnless(cmp.Or(refused, service.callers.Operator(caller)), personId)
	return Steps{}.Unless(failure, func() error { return service.devices.Forget(personId) })
}

func (service *PersonRouteService) searchUnless(refused error, query MemberQuery) (PersonPage, error) {
	if refused != nil {
		return PersonPage{Items: []PersonSummary{}}, refused
	}
	return service.people.Search(query)
}

func (service *PersonRouteService) recordUnless(refused error, personId string) (PersonRecord, error) {
	if refused != nil {
		return service.emptyRecord(), refused
	}
	return service.people.Record(personId)
}

func (service *PersonRouteService) seedUnless(refused error, seed PersonSeed) (PersonRecord, error) {
	if refused != nil {
		return service.emptyRecord(), refused
	}
	return service.seeds.Seed(seed)
}

func (service *PersonRouteService) changeUnless(refused error, caller oidc.Caller, personId string, change PersonChange) (PersonSummary, error) {
	if refused != nil {
		return PersonSummary{}, refused
	}
	return service.people.Change(caller, personId, change)
}

func (service *PersonRouteService) resetUnless(refused error, caller oidc.Caller, personId string, reset PasswordReset) error {
	if refused != nil {
		return refused
	}
	return service.people.ResetPassword(caller, personId, reset)
}

func (service *PersonRouteService) removeTotpUnless(refused error, caller oidc.Caller, personId string) error {
	if refused != nil {
		return refused
	}
	return service.people.RemoveTotp(caller, personId)
}

func (service *PersonRouteService) removePasskeysUnless(refused error, caller oidc.Caller, personId string) error {
	if refused != nil {
		return refused
	}
	return service.people.RemovePasskeys(caller, personId)
}

func (service *PersonRouteService) endSessionsUnless(refused error, personId string) error {
	if refused != nil {
		return refused
	}
	return service.people.EndSessions(personId)
}

func (service *PersonRouteService) deleteUnless(refused error, personId string) error {
	if refused != nil {
		return refused
	}
	return service.people.Delete(personId)
}

func (service *PersonRouteService) devicesAfter(summary PersonSummary) error {
	if summary.State != service.disabled() {
		return nil
	}
	return service.devices.RemoveAll(summary.PersonId)
}

func (service *PersonRouteService) disabled() string {
	return "disabled"
}

func (service *PersonRouteService) emptyRecord() PersonRecord {
	return PersonRecord{DomainIds: []string{}, AdminDomainIds: []string{}}
}

func (service *PersonRouteService) authorizationHeader() string {
	return "Authorization"
}
