// @Service
type DomainFactoryService struct {
	// @Needs
	settings *SettingsService
}

func (factory *DomainFactoryService) New(domainId string) DomainDocument {
	settings := factory.settings.Settings()
	unknown := factory.missingRecord()
	return DomainDocument{
		DomainId:     domainId,
		Name:         domainId,
		State:        factory.pending(),
		DkimSelector: settings.DkimSelector,
		Dns:          DnsState{Mx: unknown, Spf: unknown, Dkim: unknown, Dmarc: unknown},
	}
}

func (factory *DomainFactoryService) pending() string {
	return "pending"
}

func (factory *DomainFactoryService) missingRecord() string {
	return "missing"
}
