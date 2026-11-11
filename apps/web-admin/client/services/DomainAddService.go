import (
	"regexp"
	"slices"
	"strings"
)

// @Service
type DomainAddService struct {
	Open       bool
	Step       string
	Name       string
	Problem    string
	HasProblem bool
	BusyText   string
	AddedId    string
	AddedName  string
	// @Needs
	session *AdminSessionService
	// @Needs
	directoryClient *DirectoryClient
	// @Needs
	problems *AdminProblemService
	// @Needs
	adminShell *AdminShellService
	// @Needs
	domainList *DomainListService
}

func (add *DomainAddService) Show() {
	add.Open = true
	add.Step = add.form()
	add.Name = ""
	add.Problem = ""
	add.HasProblem = false
}

func (add *DomainAddService) Close() {
	add.Open = false
}

func (add *DomainAddService) AddWith(name string) {
	add.Name = strings.ToLower(strings.TrimSpace(name))
	add.Problem = add.nameProblem(add.Name)
	add.Step = add.stepFor(add.Problem, add.busy())
	add.BusyText = "Adding " + add.Name + " and making its DKIM key…"
	domain, failure := add.createWhenBusy(add.Step)
	add.Problem = add.firstProblem(add.Problem, add.answerProblem(add.problems.Of(failure)))
	add.HasProblem = add.Problem != ""
	add.Step = add.stepFor(add.Problem, add.added())
	add.AddedId = domain.DomainId
	add.AddedName = domain.Name
	add.refreshWhenAdded(add.Step)
}

func (add *DomainAddService) ShowDns() {
	add.Open = false
	add.adminShell.Open("domain", add.AddedId)
}

func (add *DomainAddService) nameProblem(name string) string {
	rules := add.rules(name)
	return rules[slices.IndexFunc(rules, add.isBroken)].Message
}

func (add *DomainAddService) rules(name string) []DomainNameRule {
	return []DomainNameRule{
		{Broken: name == "", Message: "Enter a domain name, like example.net."},
		{Broken: strings.Contains(name, " "), Message: add.notValid() + " Remove the space." + add.lettersHint()},
		{Broken: strings.Contains(name, "@"), Message: add.domainOnly()},
		{Broken: strings.HasPrefix(name, "www."), Message: add.domainOnly()},
		{Broken: !add.namePattern().MatchString(name), Message: add.notValid() + add.lettersHint()},
		{Broken: true, Message: ""},
	}
}

func (add *DomainAddService) isBroken(rule DomainNameRule) bool {
	return rule.Broken
}

func (add *DomainAddService) namePattern() *regexp.Regexp {
	return regexp.MustCompile(`^([\p{L}\p{N}]([\p{L}\p{N}-]*[\p{L}\p{N}])?\.)+[\p{L}\p{N}]([\p{L}\p{N}-]*[\p{L}\p{N}])?$`)
}

func (add *DomainAddService) stepFor(problem string, success string) string {
	if problem != "" {
		return add.form()
	}
	return success
}

func (add *DomainAddService) form() string {
	return "form"
}

func (add *DomainAddService) busy() string {
	return "busy"
}

func (add *DomainAddService) added() string {
	return "added"
}

func (add *DomainAddService) createWhenBusy(step string) (Domain, error) {
	if step != add.busy() {
		return Domain{}, nil
	}
	return add.directoryClient.CreateDomain(add.session.Authorization(), CreateDomainRequest{Name: add.Name})
}

func (add *DomainAddService) firstProblem(problem string, answerProblem string) string {
	if problem != "" {
		return problem
	}
	return answerProblem
}

func (add *DomainAddService) answerProblem(problem string) string {
	text, known := add.answerProblems()[problem]
	if !known {
		return "The domain was not added. Mesail could not reach the server. Try again."
	}
	return text
}

func (add *DomainAddService) answerProblems() map[string]string {
	return map[string]string{
		"":          "",
		"conflict":  "This domain is already added to Mesail.",
		"invalid":   add.notValid() + add.lettersHint(),
		"forbidden": "You need operator rights to add domains.",
		"signedOut": "Your session ended. Sign in again to add domains.",
	}
}

func (add *DomainAddService) refreshWhenAdded(step string) {
	if step != add.added() {
		return
	}
	add.domainList.Refresh()
}

func (add *DomainAddService) notValid() string {
	return "This is not a valid domain name."
}

func (add *DomainAddService) lettersHint() string {
	return " Use letters, digits, hyphens and dots, like example.net."
}

func (add *DomainAddService) domainOnly() string {
	return "Use the domain only, without www or @."
}
