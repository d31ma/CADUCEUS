import (
	"cmp"
	"strings"

	"mesail.dev/packages/recipients"
)

// @Service
type DomainService struct {
	// @Needs
	domains *DomainStorageService
	// @Needs
	collections *TenantCollectionsService
	// @Needs
	dns *DnsService
	// @Needs
	factory *DomainFactoryService
}

func (service *DomainService) List() ([]Domain, error) {
	documents, failure := service.domains.All()
	return service.wires(documents), failure
}

func (service *DomainService) Get(domainId string) (Domain, error) {
	document, failure := service.existing(domainId)
	return service.wire(document), failure
}

func (service *DomainService) Add(request CreateDomainRequest) (Domain, error) {
	domainId, invalid := service.name(request.Name)
	_, found, readFailure := service.domains.Find(domainId)
	failure := cmp.Or(invalid, readFailure, service.duplicate(found, domainId))
	failure = service.unless(failure, func() error { return service.collections.Prepare(domainId) })
	document := service.factory.New(domainId)
	return service.wire(document), service.unless(failure, func() error { return service.domains.Insert(document) })
}

func (service *DomainService) Verify(domainId string) (Domain, error) {
	document, failure := service.existing(domainId)
	states, checkFailure := service.checkUnless(failure, document)
	document.Dns = states
	document.State = service.stateAfterCheck(document.State, states)
	return service.wire(document), service.unless(cmp.Or(failure, checkFailure), func() error { return service.domains.Save(document) })
}

func (service *DomainService) Remove(domainId string) (Domain, error) {
	document, failure := service.existing(domainId)
	document.State = service.removing()
	return service.wire(document), service.unless(failure, func() error { return service.domains.Save(document) })
}

func (service *DomainService) existing(domainId string) (DomainDocument, error) {
	document, found, failure := service.domains.Find(strings.ToLower(domainId))
	return document, cmp.Or(failure, service.missing(found, domainId))
}

func (service *DomainService) name(raw string) (string, error) {
	domainId, invalid := recipients.DomainNames{}.Normalize(raw)
	if invalid != nil {
		return "", &InvalidRequest{Field: "name", Message: invalid.Error()}
	}
	return domainId, nil
}

func (service *DomainService) duplicate(found bool, domainId string) error {
	if found {
		return &Duplicate{Message: domainId + " is already hosted"}
	}
	return nil
}

func (service *DomainService) missing(found bool, domainId string) error {
	if found {
		return nil
	}
	return &NotFound{Message: "no hosted domain " + domainId}
}

func (service *DomainService) unless(failure error, step func() error) error {
	if failure != nil {
		return failure
	}
	return step()
}

func (service *DomainService) checkUnless(failure error, document DomainDocument) (DnsState, error) {
	if failure != nil {
		return document.Dns, nil
	}
	return service.dns.Check(document)
}

func (service *DomainService) stateAfterCheck(state string, states DnsState) string {
	if (DomainCheck{State: state, Dns: states}) == service.readyToActivate() {
		return service.active()
	}
	return state
}

func (service *DomainService) readyToActivate() DomainCheck {
	return DomainCheck{State: service.pending(), Dns: DnsState{Mx: service.ok(), Spf: service.ok(), Dkim: service.ok(), Dmarc: service.ok()}}
}

func (service *DomainService) wires(documents []DomainDocument) []Domain {
	domains := []Domain{}
	for _, document := range documents {
		domains = append(domains, service.wire(document))
	}
	return domains
}

func (service *DomainService) wire(document DomainDocument) Domain {
	return Domain{DomainId: document.DomainId, Name: document.Name, State: document.State, DkimSelector: document.DkimSelector, Dns: document.Dns}
}

func (service *DomainService) pending() string {
	return "pending"
}

func (service *DomainService) active() string {
	return "active"
}

func (service *DomainService) removing() string {
	return "removing"
}

func (service *DomainService) ok() string {
	return "ok"
}
