import "strings"

// @Service
type AliasFormService struct {
	DomainId    string
	Address     string
	MemberId    string
	Choices     []MemberChoice
	ListProblem string
	Busy        bool
	Problem     string
	Status      string
	Added       []AliasAdded
	HasAdded    bool
	memberNames map[string]string
	// @Needs
	session *AdminSessionService
	// @Needs
	directoryClient *DirectoryClient
	// @Needs
	problems                  *AdminProblemService
	applicationEventPublisher ApplicationEventPublisher
}

// @OnEvent({"condition":"value.view == 'members'","replay":true})
// @InBackground
func (form *AliasFormService) Opened(opened AdminViewOpened) {
	form.DomainId = opened.DomainId
	form.Status = ""
	page, failure := form.directoryClient.Members(form.session.Authorization(), "")
	form.Choices = form.choicesOf(page.Items)
	form.memberNames = form.namesOf(page.Items)
	form.ListProblem = form.problems.Of(failure)
	form.Problem = ""
	form.applicationEventPublisher.PublishEvent(AdminViewLoaded{View: opened.View, Message: ""}, false)
}

func (form *AliasFormService) AddWith(address string, memberId string) {
	form.Address = address
	form.MemberId = memberId
	form.Status = ""
	localPart := form.localPartOf(form.Address)
	form.Problem = form.memberProblem(form.MemberId)
	form.Problem = form.addressProblem(form.hasNoEdgeDot(localPart), form.Problem)
	form.Problem = form.addressProblem(form.usesAllowedCharacters(localPart), form.Problem)
	form.Problem = form.addressProblem(form.fitsLength(localPart), form.Problem)
	created, failure := form.createWhenValid(localPart + form.Suffix())
	form.Problem = form.firstProblem(form.Problem, form.problems.Of(failure))
	form.remember(created, form.memberNames[form.MemberId])
}

func (form *AliasFormService) choicesOf(items []MemberSummary) []MemberChoice {
	choices := []MemberChoice{}
	for _, item := range items {
		choices = append(choices, MemberChoice{MemberId: item.MemberId, Label: item.DisplayName + " (" + item.PrimaryAddress + ")"})
	}
	return choices
}

func (form *AliasFormService) namesOf(items []MemberSummary) map[string]string {
	names := map[string]string{}
	for _, item := range items {
		names[item.MemberId] = item.DisplayName
	}
	return names
}

func (form *AliasFormService) localPartOf(address string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(address)), form.Suffix())
}

func (form *AliasFormService) Suffix() string {
	return "@" + strings.ToLower(form.DomainId)
}

func (form *AliasFormService) AddressHint() string {
	return "Ends with " + form.Suffix() + ". The address must not belong to a person, an alias or another channel."
}

func (form *AliasFormService) AddressError(problem string) string {
	return form.addressErrors()[problem]
}

func (form *AliasFormService) addressErrors() map[string]string {
	return map[string]string{
		form.invalidAddress(): "Use 1 to 64 lowercase letters, digits, dots, hyphens, underscores or plus signs. Do not start or end with a dot.",
		"conflict":            "This address already belongs to a member, alias or channel.",
		"invalid":             "This is not a valid address in " + form.DomainId + form.period(),
	}
}

func (form *AliasFormService) fitsLength(localPart string) bool {
	return localPart != "" && len(localPart) <= form.longestLocalPart()
}

func (form *AliasFormService) usesAllowedCharacters(localPart string) bool {
	return strings.Trim(localPart, "abcdefghijklmnopqrstuvwxyz0123456789._+-") == ""
}

func (form *AliasFormService) hasNoEdgeDot(localPart string) bool {
	return !strings.HasPrefix(localPart, form.period()) && !strings.HasSuffix(localPart, form.period())
}

func (form *AliasFormService) memberProblem(memberId string) string {
	if memberId == "" {
		return "noMember"
	}
	return ""
}

func (form *AliasFormService) addressProblem(valid bool, previous string) string {
	if valid {
		return previous
	}
	return form.invalidAddress()
}

func (form *AliasFormService) createWhenValid(address string) (Address, error) {
	if form.Problem != "" {
		return Address{}, nil
	}
	return form.create(address)
}

func (form *AliasFormService) create(address string) (Address, error) {
	form.Busy = true
	request := CreateAliasRequest{Address: address, MemberId: form.MemberId}
	created, failure := form.directoryClient.CreateAlias(form.session.Authorization(), request)
	form.Busy = false
	return created, failure
}

func (form *AliasFormService) firstProblem(previous string, problem string) string {
	if previous != "" {
		return previous
	}
	return problem
}

func (form *AliasFormService) remember(created Address, memberName string) {
	if created.Address == "" {
		return
	}
	form.Added = append(form.Added, AliasAdded{Address: created.Address, MemberName: memberName})
	form.HasAdded = true
	form.Status = created.Address + " now reaches " + memberName + form.period()
	form.Address = ""
}

func (form *AliasFormService) invalidAddress() string {
	return "invalidAddress"
}

func (form *AliasFormService) longestLocalPart() int {
	return 64
}

func (form *AliasFormService) period() string {
	return "."
}
