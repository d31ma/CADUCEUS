import (
	"cmp"
	"net/http"

	"mesail.dev/packages/oidc"
)

// @RequestRoute("/api/v1/directory/people")
// @Allow(Operator)
type PeopleController struct {
	// @Needs
	callers *CallerService
	// @Needs
	people *PersonService
}

// @GetRoute
func (controller *PeopleController) GetPeople(request *http.Request) (PersonPage, error) {
	caller, refused := controller.callers.Caller(request.Header.Get(controller.authorizationHeader()))
	query, invalid := PageWindow{}.Query(request.URL.Query())
	return controller.searchUnless(cmp.Or(refused, controller.callers.Operator(caller), invalid), query)
}

// @GetRoute("/{person_id}")
// @Allow(Service)
func (controller *PeopleController) GetPerson(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("person_id")
	personId string,
) (PersonRecord, error) {
	caller, refused := controller.callers.Caller(authorization)
	return controller.recordUnless(cmp.Or(refused, controller.callers.InstallWide(caller)), personId)
}

// @PatchRoute("/{person_id}")
// @Allow(DomainAdmin, Operator)
func (controller *PeopleController) PatchPerson(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("person_id")
	personId string,
	// @FromBody
	change PersonChange,
) (PersonSummary, error) {
	caller, refused := controller.callers.Caller(authorization)
	return controller.changeUnless(refused, caller, personId, change)
}

// @PostRoute("/{person_id}/password")
// @Allow(DomainAdmin, Operator)
// @ResponseStatus(204)
func (controller *PeopleController) PostPassword(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("person_id")
	personId string,
	// @FromBody
	reset PasswordReset,
) error {
	caller, refused := controller.callers.Caller(authorization)
	return controller.resetUnless(refused, caller, personId, reset)
}

// @DeleteRoute("/{person_id}/totp")
// @Allow(DomainAdmin, Operator)
// @ResponseStatus(204)
func (controller *PeopleController) DeleteTotp(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("person_id")
	personId string,
) error {
	caller, refused := controller.callers.Caller(authorization)
	return controller.removeTotpUnless(refused, caller, personId)
}

// @DeleteRoute("/{person_id}")
// @ResponseStatus(204)
func (controller *PeopleController) DeletePerson(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("person_id")
	personId string,
) error {
	caller, refused := controller.callers.Caller(authorization)
	return controller.deleteUnless(cmp.Or(refused, controller.callers.Operator(caller)), personId)
}

func (controller *PeopleController) searchUnless(refused error, query MemberQuery) (PersonPage, error) {
	switch {
	case refused != nil:
		return PersonPage{Items: []PersonSummary{}}, refused
	}
	return controller.people.Search(query)
}

func (controller *PeopleController) recordUnless(refused error, personId string) (PersonRecord, error) {
	switch {
	case refused != nil:
		return PersonRecord{DomainIds: []string{}, AdminDomainIds: []string{}}, refused
	}
	return controller.people.Record(personId)
}

func (controller *PeopleController) changeUnless(refused error, caller oidc.Caller, personId string, change PersonChange) (PersonSummary, error) {
	if refused != nil {
		return PersonSummary{}, refused
	}
	return controller.people.Change(caller, personId, change)
}

func (controller *PeopleController) resetUnless(refused error, caller oidc.Caller, personId string, reset PasswordReset) error {
	if refused != nil {
		return refused
	}
	return controller.people.ResetPassword(caller, personId, reset)
}

func (controller *PeopleController) removeTotpUnless(refused error, caller oidc.Caller, personId string) error {
	if refused != nil {
		return refused
	}
	return controller.people.RemoveTotp(caller, personId)
}

func (controller *PeopleController) deleteUnless(refused error, personId string) error {
	if refused != nil {
		return refused
	}
	return controller.people.Delete(personId)
}

func (controller *PeopleController) authorizationHeader() string {
	return "Authorization"
}
