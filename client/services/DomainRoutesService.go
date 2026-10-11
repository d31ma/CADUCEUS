// DomainRoutesService lists the routing rules of the account's mail domains.
// @Service
type DomainRoutesService struct {
	// @Needs
	domainsClient             *DomainsClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	Domains                   []MailDomain
}

// @OnStart
func (service *DomainRoutesService) Start() {
	service.Domains = []MailDomain{}
}

// @OnEvent({"replay":true})
func (service *DomainRoutesService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
	FlowUtils{}.When(changed.SignedIn, service.load)()
}

func (service *DomainRoutesService) load() {
	domains, failure := service.domainsClient.Domains(service.authorization)
	service.Domains = append([]MailDomain{}, domains...)
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "domain-routes", Refused: HTTPUtils{}.Refused(failure)}, false)
}
