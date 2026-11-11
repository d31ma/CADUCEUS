import "slices"

// @Service
type DomainDnsService struct {
	Step         string
	DomainId     string
	Name         string
	StateText    string
	StateColor   string
	StateVariant string
	StateIcon    string
	Pending      bool
	Waiting      string
	Status       string
	Records      []DnsRecord
	Checking     bool
	CheckProblem string
	ProblemText  string
	domain       Domain
	// @Needs
	session *AdminSessionService
	// @Needs
	directoryClient *DirectoryClient
	// @Needs
	problems *AdminProblemService
	// @Needs
	domainRows                *DomainRowService
	applicationEventPublisher ApplicationEventPublisher
}

// @OnEvent({"condition":"value.view == 'domain'","replay":true})
// @InBackground
func (dns *DomainDnsService) Opened(opened AdminViewOpened) {
	dns.load(opened.DomainId)
	dns.applicationEventPublisher.PublishEvent(AdminViewLoaded{View: opened.View, Message: dns.Status}, false)
}

func (dns *DomainDnsService) Refresh() {
	dns.load(dns.DomainId)
}

func (dns *DomainDnsService) Check() {
	dns.Checking = true
	domain, failure := dns.directoryClient.VerifyDomain(dns.session.Authorization(), dns.DomainId)
	dns.show(dns.keptWhenFailed(domain, failure))
	dns.CheckProblem = dns.checkProblem(dns.problems.Of(failure))
	dns.Checking = false
}

func (dns *DomainDnsService) load(domainId string) {
	dns.DomainId = domainId
	dns.Step = "loading"
	dns.CheckProblem = ""
	domains, failure := dns.directoryClient.Domains(dns.session.Authorization())
	domain := dns.find(domains, domainId)
	problem := dns.problems.Of(failure)
	dns.show(domain)
	dns.ProblemText = dns.loadProblem(problem)
	dns.Step = dns.firstOf(dns.problemStep(problem), dns.foundStep(domain.DomainId))
}

func (dns *DomainDnsService) show(domain Domain) {
	view := dns.domainRows.View(domain)
	dns.domain = domain
	dns.Name = dns.firstOf(domain.Name, dns.DomainId)
	dns.StateText = view.StateText
	dns.StateColor = view.StateColor
	dns.StateVariant = view.StateVariant
	dns.StateIcon = view.StateIcon
	dns.Pending = view.Pending
	dns.Waiting = view.Waiting
	dns.Status = view.Status
	dns.Records = view.Records
}

func (dns *DomainDnsService) find(domains []Domain, domainId string) Domain {
	index := slices.IndexFunc(domains, func(domain Domain) bool { return domain.DomainId == domainId })
	if index < 0 {
		return Domain{}
	}
	return domains[index]
}

func (dns *DomainDnsService) keptWhenFailed(domain Domain, failure error) Domain {
	if failure != nil {
		return dns.domain
	}
	return domain
}

func (dns *DomainDnsService) firstOf(first string, second string) string {
	if first != "" {
		return first
	}
	return second
}

func (dns *DomainDnsService) foundStep(domainId string) string {
	if domainId == "" {
		return dns.notFound()
	}
	return "ready"
}

func (dns *DomainDnsService) problemStep(problem string) string {
	step, known := dns.problemSteps()[problem]
	if !known {
		return "failed"
	}
	return step
}

func (dns *DomainDnsService) problemSteps() map[string]string {
	return map[string]string{"": "", dns.forbidden(): "operatorsOnly", dns.notFound(): dns.notFound()}
}

func (dns *DomainDnsService) loadProblem(problem string) string {
	text, known := dns.loadProblems()[problem]
	if !known {
		return dns.unreachable()
	}
	return text
}

func (dns *DomainDnsService) loadProblems() map[string]string {
	return map[string]string{
		"":              "",
		dns.forbidden(): "Only operators can see DNS checks for now. Ask a Mesail operator to check the DNS records of this domain.",
		dns.notFound():  "This domain was not found. It may have been removed.",
	}
}

func (dns *DomainDnsService) checkProblem(problem string) string {
	text, known := dns.checkProblems()[problem]
	if !known {
		return "The records were not checked. " + dns.unreachable()
	}
	return text
}

func (dns *DomainDnsService) checkProblems() map[string]string {
	return map[string]string{"": "", dns.forbidden(): "Only operators can check DNS records."}
}

func (dns *DomainDnsService) unreachable() string {
	return "Mesail could not reach the server. Check your connection, then try again."
}

func (dns *DomainDnsService) forbidden() string {
	return "forbidden"
}

func (dns *DomainDnsService) notFound() string {
	return "notFound"
}
