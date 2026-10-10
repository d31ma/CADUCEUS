// DomainRoutesService lists the routing rules of the account's mail domains.
// @Service
type DomainRoutesService struct {
	// @Needs
	domainsClient             *DomainsClient
	applicationEventPublisher ApplicationEventPublisher
	Domains                   []MailDomain
}

// @OnEvent({"replay":true})
func (service *DomainRoutesService) Open(session AccountSession) error {
	domains, _ := service.domainsClient.Domains("Bearer " + session.Token)
	service.Domains = append([]MailDomain{}, domains...)
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "domain-routes"}, false)
	return nil
}
