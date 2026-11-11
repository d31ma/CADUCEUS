// @Service
type MemberRoleService struct {
	Dialog       string
	MemberId     string
	Name         string
	Address      string
	DomainId     string
	Busy         bool
	LastAdminFor string
	Problem      string
	// @Needs
	session *AdminSessionService
	// @Needs
	directoryClient *DirectoryClient
	// @Needs
	problems                  *AdminProblemService
	applicationEventPublisher ApplicationEventPublisher
}

func (role *MemberRoleService) SetRole(memberId string, name string, admin bool) {
	role.pick(memberId, name, "")
	role.Busy = true
	_, failure := role.directoryClient.ChangeMember(role.session.Authorization(), memberId, MemberChange{Admin: admin})
	role.Busy = false
	role.finish(role.problems.Of(failure), "role", "")
}

func (role *MemberRoleService) AskRemove(memberId string, name string, address string) {
	role.pick(memberId, name, address)
	role.Dialog = "remove"
}

func (role *MemberRoleService) Remove() {
	role.Busy = true
	_, failure := role.directoryClient.RemoveMember(role.session.Authorization(), role.MemberId)
	role.Busy = false
	role.finish(role.removalProblem(role.problems.Of(failure)), "removal", "removed")
}

func (role *MemberRoleService) Close() {
	role.Dialog = ""
}

func (role *MemberRoleService) pick(memberId string, name string, address string) {
	role.MemberId = memberId
	role.Name = name
	role.Address = address
	role.DomainId = role.session.DomainId
	role.Problem = ""
}

func (role *MemberRoleService) finish(problem string, change string, doneDialog string) {
	role.Dialog = role.dialogAfter(problem, doneDialog)
	role.LastAdminFor = change
	role.Problem = problem
	role.announce(problem)
}

func (role *MemberRoleService) removalProblem(problem string) string {
	if problem == "notFound" {
		return ""
	}
	return problem
}

func (role *MemberRoleService) dialogAfter(problem string, doneDialog string) string {
	dialog, known := map[string]string{"": doneDialog, "conflict": "lastAdmin", "forbidden": "forbidden"}[problem]
	if !known {
		return "failed"
	}
	return dialog
}

func (role *MemberRoleService) announce(problem string) {
	if problem != "" {
		return
	}
	role.applicationEventPublisher.PublishEvent(PeopleChanged{PersonId: role.MemberId}, false)
}
