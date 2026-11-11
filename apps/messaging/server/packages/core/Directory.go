package core

type Directory interface {
	Resolve(authorization string, addresses []string) ([]Resolved, error)
	Address(domainId string, address string) (AddressEntry, bool, error)
}
