import "net/http"

// @RequestRoute("/api/v1/directory/members")
// @SignedIn
type MembersController struct {
	// @Needs
	admission *AdmissionService
	// @Needs
	members *MemberService
	// @Needs
	memberships *MembershipService
}

// @GetRoute
func (controller *MembersController) GetMembers(request *http.Request) (MemberPage, error) {
	admission := controller.admission.Admit(request.Header.Get(controller.authorizationHeader()))
	query, invalid := PageWindow{}.Query(request.URL.Query())
	return controller.members.Search(admission, query, invalid)
}

// @GetRoute("/{member_id}")
func (controller *MembersController) GetMember(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("member_id")
	memberId string,
) (MemberSummary, error) {
	return controller.members.Summary(controller.admission.Admit(authorization), memberId)
}

// @PostRoute
// @Allow(DomainAdmin)
// @ResponseStatus(201)
func (controller *MembersController) PostMember(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request AddMemberRequest,
) (AddedMember, error) {
	return controller.postMemberUnless(controller.admission.Admit(authorization), request)
}

// @PatchRoute("/{member_id}")
// @Allow(DomainAdmin)
func (controller *MembersController) PatchMember(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("member_id")
	memberId string,
	// @FromBody
	change MemberChange,
) (MemberSummary, error) {
	return controller.patchMemberUnless(controller.admission.Admit(authorization), memberId, change)
}

// @DeleteRoute("/{member_id}")
// @Allow(DomainAdmin)
// @ResponseStatus(204)
func (controller *MembersController) DeleteMember(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("member_id")
	memberId string,
) error {
	return controller.deleteMemberUnless(controller.admission.Admit(authorization), memberId)
}

func (controller *MembersController) postMemberUnless(admission Admission, request AddMemberRequest) (AddedMember, error) {
	if admission.Refusal != nil {
		return AddedMember{}, admission.Refusal
	}
	return controller.memberships.Add(admission.Caller, admission.Caller.DomainId, request)
}

func (controller *MembersController) patchMemberUnless(admission Admission, memberId string, change MemberChange) (MemberSummary, error) {
	if admission.Refusal != nil {
		return MemberSummary{}, admission.Refusal
	}
	return controller.memberships.Change(admission.Caller, memberId, change)
}

func (controller *MembersController) deleteMemberUnless(admission Admission, memberId string) error {
	if admission.Refusal != nil {
		return admission.Refusal
	}
	return controller.memberships.Remove(admission.Caller, memberId)
}

func (controller *MembersController) authorizationHeader() string {
	return "Authorization"
}
