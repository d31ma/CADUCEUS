import "net/http"

// @RequestRoute("/api/v1/directory/people")
// @Allow(Operator)
type PeopleController struct {
	// @Needs
	people *PersonRouteService
}

// @GetRoute
func (controller *PeopleController) GetPeople(request *http.Request) (PersonPage, error) {
	return controller.people.Search(request)
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
	return controller.people.RemoveTotp(authorization, personId)
}

// @GetRoute("/{person_id}")
// @Allow(Service)
func (controller *PeopleController) GetPerson(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("person_id")
	personId string,
) (PersonRecord, error) {
	return controller.people.Record(authorization, personId)
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
	return controller.people.ResetPassword(authorization, personId, reset)
}

// @DeleteRoute("/{person_id}/passkeys")
// @Allow(DomainAdmin, Operator)
// @ResponseStatus(204)
func (controller *PeopleController) DeletePasskeys(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("person_id")
	personId string,
) error {
	return controller.people.RemovePasskeys(authorization, personId)
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
	return controller.people.Change(authorization, personId, change)
}

// @DeleteRoute("/{person_id}")
// @ResponseStatus(202)
func (controller *PeopleController) DeletePerson(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("person_id")
	personId string,
) error {
	return controller.people.Delete(authorization, personId)
}

// @PostRoute("/seed")
// @Allow(Service)
func (controller *PeopleController) PostSeed(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	seed PersonSeed,
) (PersonRecord, error) {
	return controller.people.Seed(authorization, seed)
}
