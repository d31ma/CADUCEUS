import "net/http"

// @RequestRoute("/api/v1/directory/members")
// @SignedIn
type MembersController struct {
	// @Needs
	members *MemberRouteService
}

// @GetRoute
func (controller *MembersController) GetMembers(request *http.Request) (MemberPage, error) {
	return controller.members.Search(request)
}

// @GetRoute("/{member_id}")
func (controller *MembersController) GetMember(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("member_id")
	memberId string,
) (MemberSummary, error) {
	return controller.members.Summary(authorization, memberId)
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
	return controller.members.Remove(authorization, memberId)
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
	return controller.members.Add(authorization, request)
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
	return controller.members.Change(authorization, memberId, change)
}
