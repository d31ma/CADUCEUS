import (
	"time"

	"mesail.dev/packages/dnscheck"
	"mesail.dev/packages/transport"
)

// @Service
type DnsService struct {
	// @Needs
	settings *SettingsService
}

func (service *DnsService) Check(domain DomainDocument) (DnsState, error) {
	settings := service.settings.Settings()
	checker := dnscheck.NewChecker(service.lookup(settings.DnsResolver))
	expected := dnscheck.Expected{MailHost: settings.MailHost, SpfMechanism: settings.SpfMechanism, DkimSelector: domain.DkimSelector}
	states, failure := checker.Check(domain.DomainId, expected)
	return DnsState{Mx: states.Mx, Spf: states.Spf, Dkim: states.Dkim, Dmarc: states.Dmarc}, service.unavailable(failure)
}

func (service *DnsService) lookup(resolver string) dnscheck.Lookup {
	if resolver == "" {
		return dnscheck.NewSystemLookup(service.timeout())
	}
	return dnscheck.NewDohLookup(resolver, transport.SingleUseTransport{}.Client(service.timeout()))
}

func (service *DnsService) unavailable(failure error) error {
	if failure == nil {
		return nil
	}
	return &Unavailable{Message: "the DNS lookup failed", Cause: failure}
}

func (service *DnsService) timeout() time.Duration {
	return 30 * time.Second
}
