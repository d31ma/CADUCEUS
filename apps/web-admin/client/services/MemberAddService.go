import "strings"

// @Service
type MemberAddService struct {
	Step            string
	DomainId        string
	FirstAdmin      bool
	Username        string
	Admin           bool
	DisplayName     string
	UsernameProblem string
	NameProblem     string
	PasswordProblem string
	Problem         string
	AddedName       string
	AddedAddress    string
	AddedFirstName  string
	ExistingPerson  bool
	// @Needs
	session *AdminSessionService
	// @Needs
	directoryClient *DirectoryClient
	// @Needs
	problems *AdminProblemService
	// @Needs
	rules *UsernameRuleService
	applicationEventPublisher ApplicationEventPublisher
}

func (add *MemberAddService) Open() {
	add.start(add.session.DomainId, false, "")
	add.Step = add.usernameStep()
}

func (add *MemberAddService) OpenWith(username string) {
	add.start(add.session.DomainId, false, username)
	add.Step = add.usernameStep()
}

func (add *MemberAddService) NameFirstAdmin(domainId string, username string) {
	add.start(domainId, true, username)
	add.AddWith(username, true)
}

func (add *MemberAddService) AddWith(username string, admin bool) {
	add.Username = add.rules.Cleaned(username)
	add.Admin = admin
	add.Problem = ""
	add.UsernameProblem = add.firstOf(add.rules.Problem(username), add.rules.ListedProblem(add.Username, add.listedDomain()))
	add.sendWhenValid(add.UsernameProblem, AddMemberRequest{Username: add.Username, Admin: admin})
}

func (add *MemberAddService) CreateWith(displayName string, password string) {
	add.DisplayName = strings.TrimSpace(displayName)
	add.Problem = ""
	add.NameProblem = add.nameProblem(add.DisplayName)
	add.PasswordProblem = add.rules.PasswordProblem(password)
	request := AddMemberRequest{Username: add.Username, DisplayName: add.DisplayName, FirstPassword: password, Admin: add.Admin}
	add.createWhenValid(add.NameProblem+add.PasswordProblem, request)
}

func (add *MemberAddService) Back() {
	add.Step = add.usernameStep()
	add.NameProblem = ""
	add.PasswordProblem = ""
}

func (add *MemberAddService) AddAnother() {
	add.Open()
}

func (add *MemberAddService) Close() {
	add.Step = ""
}

func (add *MemberAddService) start(domainId string, firstAdmin bool, username string) {
	add.DomainId = domainId
	add.FirstAdmin = firstAdmin
	add.Username = username
	add.Admin = firstAdmin
	add.DisplayName = ""
	add.UsernameProblem = ""
	add.NameProblem = ""
	add.PasswordProblem = ""
	add.Problem = ""
}

func (add *MemberAddService) sendWhenValid(problem string, request AddMemberRequest) {
	if problem != "" {
		add.Step = add.usernameStep()
		return
	}
	add.Step = "busy"
	answer, failure := add.send(request)
	add.answered(answer, add.problems.Of(failure), add.usernameStep())
}

func (add *MemberAddService) createWhenValid(problem string, request AddMemberRequest) {
	if problem != "" {
		return
	}
	add.Step = "creating"
	answer, failure := add.send(request)
	add.answered(answer, add.problems.Of(failure), add.detailsStep())
}

func (add *MemberAddService) send(request AddMemberRequest) (AddedMember, error) {
	return add.senders()[add.FirstAdmin](request)
}

func (add *MemberAddService) senders() map[bool]func(AddMemberRequest) (AddedMember, error) {
	return map[bool]func(AddMemberRequest) (AddedMember, error){
		true: func(request AddMemberRequest) (AddedMember, error) {
			return add.directoryClient.AddDomainMember(add.session.Authorization(), add.DomainId, request)
		},
		false: func(request AddMemberRequest) (AddedMember, error) {
			return add.directoryClient.AddMember(add.session.Authorization(), request)
		},
	}
}

func (add *MemberAddService) listedDomain() string {
	return map[bool]string{true: "", false: add.DomainId}[add.FirstAdmin]
}

func (add *MemberAddService) answered(answer AddedMember, problem string, step string) {
	add.Step = add.stepAfter(problem, step)
	add.UsernameProblem = add.conflictProblem(problem, add.UsernameProblem)
	add.PasswordProblem = add.policyProblem(problem, step, add.PasswordProblem)
	add.Problem = add.otherProblem(problem)
	add.showAdded(answer, problem)
}

func (add *MemberAddService) stepAfter(problem string, step string) string {
	next, known := map[string]string{"": "added", add.invalid(): add.detailsStep(), add.conflict(): add.usernameStep()}[problem]
	if !known {
		return step
	}
	return next
}

func (add *MemberAddService) conflictProblem(problem string, current string) string {
	if problem != add.conflict() {
		return current
	}
	return add.Username + " cannot be used. It is already a member here, or an alias or channel address uses it in a domain on this install."
}

func (add *MemberAddService) policyProblem(problem string, step string, current string) string {
	if problem != add.invalid() || step != add.detailsStep() {
		return current
	}
	return "Mesail did not accept this password. Use at least 12 characters, and not the username or the address."
}

func (add *MemberAddService) otherProblem(problem string) string {
	if map[string]bool{"": true, add.invalid(): true, add.conflict(): true}[problem] {
		return ""
	}
	return problem
}

func (add *MemberAddService) showAdded(answer AddedMember, problem string) {
	if problem != "" {
		return
	}
	add.AddedName = answer.Member.DisplayName
	add.AddedAddress = answer.Member.PrimaryAddress
	add.AddedFirstName = strings.Fields(answer.Member.DisplayName + " " + add.Username)[0]
	add.ExistingPerson = answer.ExistingPerson
	add.applicationEventPublisher.PublishEvent(PeopleChanged{PersonId: answer.Member.MemberId}, false)
}

func (add *MemberAddService) nameProblem(displayName string) string {
	if displayName != "" {
		return ""
	}
	return "Type the name people see, like Dan Example."
}

func (add *MemberAddService) firstOf(first string, second string) string {
	if first != "" {
		return first
	}
	return second
}

func (add *MemberAddService) invalid() string {
	return "invalid"
}

func (add *MemberAddService) conflict() string {
	return "conflict"
}

func (add *MemberAddService) usernameStep() string {
	return "username"
}

func (add *MemberAddService) detailsStep() string {
	return "details"
}
