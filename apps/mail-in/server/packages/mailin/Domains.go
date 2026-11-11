package mailin

type Domains interface {
	Domain(domainId string) (Domain, bool, error)
}
