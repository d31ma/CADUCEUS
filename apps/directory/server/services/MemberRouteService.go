import "net/http"

// @Service
type MemberRouteService struct {
	// @Needs
	admission *AdmissionService
	// @Needs
	members *MemberService
	// @Needs
	memberships *MembershipService
}

func (service *MemberRouteService) Search(request *http.Request) (MemberPage, error) {
	admission := service.admission.Admit(request.Header.Get(service.authorizationHeader()))
	query, invalid := PageWindow{}.Query(request.URL.Query())
	return service.members.Search(admission, query, invalid)
}

func (service *MemberRouteService) Summary(authorization string, memberId string) (MemberSummary, error) {
	return service.members.Summary(service.admission.Admit(authorization), memberId)
}

func (service *MemberRouteService) Add(authorization string, request AddMemberRequest) (AddedMember, error) {
	return service.addUnless(service.admission.Admit(authorization), request)
}

func (service *MemberRouteService) Change(authorization string, memberId string, change MemberChange) (MemberSummary, error) {
	return service.changeUnless(service.admission.Admit(authorization), memberId, change)
}

func (service *MemberRouteService) Remove(authorization string, memberId string) error {
	return service.removeUnless(service.admission.Admit(authorization), memberId)
}

func (service *MemberRouteService) addUnless(admission Admission, request AddMemberRequest) (AddedMember, error) {
	if admission.Refusal != nil {
		return AddedMember{}, admission.Refusal
	}
	return service.memberships.Add(admission.Caller, admission.Caller.DomainId, request)
}

func (service *MemberRouteService) changeUnless(admission Admission, memberId string, change MemberChange) (MemberSummary, error) {
	if admission.Refusal != nil {
		return MemberSummary{}, admission.Refusal
	}
	return service.memberships.Change(admission.Caller, memberId, change)
}

func (service *MemberRouteService) removeUnless(admission Admission, memberId string) error {
	if admission.Refusal != nil {
		return admission.Refusal
	}
	return service.memberships.Remove(admission.Caller, memberId)
}

func (service *MemberRouteService) authorizationHeader() string {
	return "Authorization"
}
