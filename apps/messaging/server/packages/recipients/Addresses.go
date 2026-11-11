package recipients

import "strings"

type Addresses struct {
	domains DomainNames
}

func (addresses Addresses) Normalize(raw string) (string, error) {
	address := strings.ToLower(strings.TrimSpace(raw))
	local, domain, found := strings.Cut(address, addresses.at())
	_, domainError := addresses.domains.Normalize(domain)
	return address, addresses.refusal(addresses.valid(found, local, domainError), raw)
}

func (addresses Addresses) Domain(address string) string {
	_, domain, _ := strings.Cut(address, addresses.at())
	return domain
}

func (addresses Addresses) valid(found bool, local string, domainError error) bool {
	return found && domainError == nil && addresses.validLocal(local)
}

func (addresses Addresses) validLocal(local string) bool {
	return local != "" && len(local) <= addresses.maxLocalLength() && !strings.ContainsAny(local, addresses.forbiddenInLocal())
}

func (addresses Addresses) refusal(valid bool, raw string) error {
	if valid {
		return nil
	}
	return InvalidAddress{Address: raw}
}

func (Addresses) at() string {
	return "@"
}

func (Addresses) forbiddenInLocal() string {
	return "@ \t\r\n<>()[],;:\\\""
}

func (Addresses) maxLocalLength() int {
	return 64
}
