import (
	"cmp"
	"errors"
	"slices"
	"strings"

	"mesail.dev/packages/oidc"
)

// @Service
type MemberService struct {
	// @Needs
	members *MemberStorageService
	// @Needs
	people *PeopleStorageService
	// @Needs
	access *PersonAccessService
}

func (service *MemberService) Search(admission Admission, query MemberQuery, invalid error) (MemberPage, error) {
	size, sizeInvalid := PageWindow{}.Size(query.Limit)
	offset, cursorInvalid := PageWindow{}.Offset(query.Cursor)
	refusal := cmp.Or(admission.Refusal, invalid, sizeInvalid, cursorInvalid)
	members, failure := service.allUnless(refusal, admission.Caller.DomainId)
	matching := service.matching(members, query.Text)
	end := min(offset+size, len(matching))
	window := matching[min(offset, end):end]
	items, itemsFailure := service.adminUnless(service.administers(admission.Caller), admission.Caller, window, service.plain(window))
	return MemberPage{Items: items, NextCursor: PageWindow{}.Next(end, len(matching))}, cmp.Or(refusal, failure, itemsFailure)
}

func (service *MemberService) Summary(admission Admission, memberId string) (MemberSummary, error) {
	member, found, failure := service.memberUnless(admission, memberId)
	return MemberSummaries{}.Plain(member), cmp.Or(admission.Refusal, failure, service.knownActive(found, member.State))
}

func (service *MemberService) memberUnless(admission Admission, memberId string) (MemberDocument, bool, error) {
	if admission.Refusal != nil {
		return MemberDocument{}, false, nil
	}
	return service.members.Find(admission.Caller.DomainId, memberId)
}

func (service *MemberService) knownActive(found bool, state string) error {
	switch {
	case !found, state != service.active():
		return &NotFound{Message: "no such member in your domain"}
	}
	return nil
}

func (service *MemberService) allUnless(refusal error, domainId string) ([]MemberDocument, error) {
	if refusal != nil {
		return []MemberDocument{}, nil
	}
	return service.members.All(domainId)
}

func (service *MemberService) adminUnless(administers bool, caller oidc.Caller, members []MemberDocument, plain []MemberSummary) ([]MemberSummary, error) {
	if !administers {
		return plain, nil
	}
	return service.adminSummaries(caller, members)
}

func (service *MemberService) adminSummaries(caller oidc.Caller, members []MemberDocument) ([]MemberSummary, error) {
	manager, failure := service.access.ManagerOf(caller)
	items, itemsFailure := service.forAdmin(manager, members)
	return items, cmp.Or(failure, itemsFailure)
}

func (service *MemberService) plain(members []MemberDocument) []MemberSummary {
	summaries := []MemberSummary{}
	for _, member := range members {
		summaries = append(summaries, MemberSummaries{}.Plain(member))
	}
	return summaries
}

func (service *MemberService) forAdmin(manager AccountManager, members []MemberDocument) ([]MemberSummary, error) {
	summaries := []MemberSummary{}
	failures := []error{}
	for _, member := range members {
		person, _, failure := service.people.ByPersonId(member.MemberId)
		summaries = append(summaries, MemberSummaries{}.Admin(member, manager.Manages(person.DomainIds)))
		failures = append(failures, failure)
	}
	return summaries, errors.Join(failures...)
}

func (service *MemberService) administers(caller oidc.Caller) bool {
	return slices.ContainsFunc(caller.Roles, func(role string) bool { return slices.Contains(service.adminRoles(), role) })
}

func (service *MemberService) matching(members []MemberDocument, query string) []MemberDocument {
	wanted := strings.ToLower(strings.TrimSpace(query))
	return slices.DeleteFunc(members, func(member MemberDocument) bool { return service.hidden(member, wanted) })
}

func (service *MemberService) hidden(member MemberDocument, wanted string) bool {
	text := strings.ToLower(strings.Join([]string{member.Username, member.DisplayName, member.PrimaryAddress}, service.separator()))
	switch {
	case member.State == service.removed(), !strings.Contains(text, wanted):
		return true
	}
	return false
}

func (service *MemberService) separator() string {
	return " "
}

func (service *MemberService) adminRoles() []string {
	return []string{"DomainAdmin", "Operator"}
}

func (service *MemberService) active() string {
	return "active"
}

func (service *MemberService) removed() string {
	return "removed"
}
