package devicebox

import (
	"time"

	"mesail.dev/packages/oidc"
)

type Expiry struct {
	callers Callers
	parts   ExpiryParts
	now     func() time.Time
}

func (expiry Expiry) Run(authorization string) (ExpireReport, error) {
	caller, err := expiry.callers.Service(authorization)
	if err != nil {
		return ExpireReport{}, err
	}
	records, err := expiry.parts.Store.Domains()
	if err != nil {
		return ExpireReport{}, err
	}
	run := NewExpiryRun(expiry.parts, expiry.now())
	return run.Domains(expiry.domainsFor(caller, records))
}

func (expiry Expiry) domainsFor(caller oidc.Caller, records []RegistryRecord) []string {
	domains := []string{}
	for _, record := range records {
		domains = append(domains, expiry.ifCovered(caller.DomainId, record.DomainId)...)
	}
	return domains
}

func (Expiry) ifCovered(callerDomain string, domainId string) []string {
	if callerDomain != domainId && !(oidc.InstallWide{}).Matches(callerDomain) {
		return nil
	}
	return []string{domainId}
}
