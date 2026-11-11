import (
	"net/http"

	"mesail.dev/packages/dnscheck"
)

// @Service
type DnsService struct {
	// @Needs
	settings *SettingsService
}

func (service *DnsService) Check(domain DomainDocument) (DnsState, error) {
	settings := service.settings.Settings()
	checker := dnscheck.NewChecker(dnscheck.NewDohLookup(settings.DnsResolver, http.DefaultClient))
	expected := dnscheck.Expected{MailHost: settings.MailHost, SpfMechanism: settings.SpfMechanism, DkimSelector: domain.DkimSelector}
	states, failure := checker.Check(domain.DomainId, expected)
	return DnsState{Mx: states.Mx, Spf: states.Spf, Dkim: states.Dkim, Dmarc: states.Dmarc}, service.unavailable(failure)
}

func (service *DnsService) unavailable(failure error) error {
	if failure == nil {
		return nil
	}
	return &Unavailable{Message: "the DNS lookup failed", Cause: failure}
}
