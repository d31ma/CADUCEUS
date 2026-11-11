import (
	"cmp"
	"slices"
	"strings"

	"mesail.dev/packages/oidc"
)

// @Service
type PersonService struct {
	// @Needs
	people *PeopleStorageService
	// @Needs
	access *PersonAccessService
	// @Needs
	accounts *AccountAdminService
	// @Needs
	writer *MemberWriterService
}

func (service *PersonService) Record(personId string) (PersonRecord, error) {
	person, failure := service.known(personId)
	return PersonRecord{PersonId: person.PersonId, Username: person.Username, DisplayName: person.DisplayName, DomainIds: service.list(person.DomainIds), AdminDomainIds: service.list(person.AdminDomainIds), DefaultDomainId: person.DefaultDomainId, State: person.State}, failure
}

func (service *PersonService) Search(query MemberQuery) (PersonPage, error) {
	size, sizeInvalid := PageWindow{}.Size(query.Limit)
	offset, cursorInvalid := PageWindow{}.Offset(query.Cursor)
	people, failure := service.people.All()
	matching := service.matching(people, query.Text)
	end := min(offset+size, len(matching))
	page := PersonPage{Items: service.summaries(matching[min(offset, end):end]), NextCursor: PageWindow{}.Next(end, len(matching))}
	return page, cmp.Or(sizeInvalid, cursorInvalid, failure)
}

func (service *PersonService) Change(caller oidc.Caller, personId string, change PersonChange) (PersonSummary, error) {
	person, failure := service.managed(caller, personId)
	changed, invalid := service.changed(person, change)
	failure = service.enableUnless(cmp.Or(failure, invalid), person.State, changed)
	stored, failure := service.storeUnless(failure, changed)
	return service.summary(stored), Steps{}.Unless(failure, func() error { return service.writer.MirrorAll(stored) })
}

func (service *PersonService) ResetPassword(caller oidc.Caller, personId string, reset PasswordReset) error {
	person, failure := service.managed(caller, personId)
	return Steps{}.Unless(cmp.Or(failure, service.passwordGiven(reset.NewPassword)), func() error { return service.accounts.ResetPassword(person.PersonId, reset.NewPassword) })
}

func (service *PersonService) RemoveTotp(caller oidc.Caller, personId string) error {
	person, failure := service.managed(caller, personId)
	return Steps{}.Unless(failure, func() error { return service.accounts.RemoveTotp(person.PersonId) })
}

func (service *PersonService) Delete(personId string) error {
	person, failure := service.known(personId)
	failure = Steps{}.Unless(failure, func() error { return service.accounts.Delete(person.PersonId) })
	failure = Steps{}.Unless(failure, func() error { return service.writer.LeaveAll(person) })
	_, failure = service.storeUnless(failure, service.deletedPerson(person))
	return failure
}

func (service *PersonService) SetDefault(caller oidc.Caller, request DefaultDomain) error {
	person, failure := service.known(caller.MemberId)
	person.DefaultDomainId = strings.ToLower(strings.TrimSpace(request.DomainId))
	_, failure = service.storeUnless(cmp.Or(failure, service.memberOf(person, person.DefaultDomainId)), person)
	return failure
}

func (service *PersonService) known(personId string) (PersonDocument, error) {
	person, found, failure := service.people.ByPersonId(personId)
	return person, cmp.Or(failure, service.present(found))
}

func (service *PersonService) managed(caller oidc.Caller, personId string) (PersonDocument, error) {
	person, failure := service.known(personId)
	return person, Steps{}.Unless(cmp.Or(failure, service.notDeleted(person)), func() error { return service.access.Check(caller, person) })
}

func (service *PersonService) changed(person PersonDocument, change PersonChange) (PersonDocument, error) {
	person.DisplayName = cmp.Or(strings.TrimSpace(change.DisplayName), person.DisplayName)
	person.State = cmp.Or(change.State, person.State)
	return person, service.validState(person.State)
}

func (service *PersonService) enableUnless(failure error, before string, person PersonDocument) error {
	switch {
	case failure != nil:
		return failure
	case before == person.State:
		return nil
	}
	return service.accounts.SetEnabled(person.PersonId, person.State == service.active())
}

func (service *PersonService) storeUnless(failure error, person PersonDocument) (PersonDocument, error) {
	if failure != nil {
		return person, failure
	}
	return service.people.Store(person)
}

func (service *PersonService) deletedPerson(person PersonDocument) PersonDocument {
	person.DomainIds = []string{}
	person.AdminDomainIds = []string{}
	person.DefaultDomainId = ""
	person.State = service.deleted()
	return person
}

func (service *PersonService) validState(state string) error {
	if !slices.Contains([]string{service.active(), service.disabled()}, state) {
		return &InvalidRequest{Field: "state", Message: "state must be active or disabled"}
	}
	return nil
}

func (service *PersonService) passwordGiven(password string) error {
	if strings.TrimSpace(password) == "" {
		return &InvalidRequest{Field: "newPassword", Message: "a new password is required"}
	}
	return nil
}

func (service *PersonService) memberOf(person PersonDocument, domainId string) error {
	if !(PersonRules{}).Member(person, domainId) {
		return &NotFound{Message: "you are not a member of " + domainId}
	}
	return nil
}

func (service *PersonService) present(found bool) error {
	if !found {
		return &NotFound{Message: "no such person"}
	}
	return nil
}

func (service *PersonService) notDeleted(person PersonDocument) error {
	if person.State == service.deleted() {
		return &NotFound{Message: "this person was deleted"}
	}
	return nil
}

func (service *PersonService) matching(people []PersonDocument, query string) []PersonDocument {
	wanted := strings.ToLower(strings.TrimSpace(query))
	return slices.DeleteFunc(people, func(person PersonDocument) bool {
		return !strings.Contains(strings.ToLower(strings.Join([]string{person.Username, person.DisplayName}, service.separator())), wanted)
	})
}

func (service *PersonService) summaries(people []PersonDocument) []PersonSummary {
	summaries := []PersonSummary{}
	for _, person := range people {
		summaries = append(summaries, service.summary(person))
	}
	return summaries
}

func (service *PersonService) summary(person PersonDocument) PersonSummary {
	return PersonSummary{PersonId: person.PersonId, Username: person.Username, DisplayName: person.DisplayName, DomainIds: service.list(person.DomainIds), State: person.State}
}

func (service *PersonService) list(values []string) []string {
	return append([]string{}, values...)
}

func (service *PersonService) separator() string {
	return " "
}

func (service *PersonService) active() string {
	return "active"
}

func (service *PersonService) disabled() string {
	return "disabled"
}

func (service *PersonService) deleted() string {
	return "deleted"
}
