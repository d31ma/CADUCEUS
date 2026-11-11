import "mesail.dev/packages/collections"

// @Service
type CollectionNamesService struct {
}

func (names *CollectionNamesService) Domains() string {
	return "domains"
}

func (names *CollectionNamesService) People() string {
	return "people"
}

func (names *CollectionNamesService) LocalParts() string {
	return "local-parts"
}

func (names *CollectionNamesService) Members(domainId string) string {
	return collections.CollectionName{}.ForDomain("members", domainId)
}

func (names *CollectionNamesService) Addresses(domainId string) string {
	return collections.CollectionName{}.ForDomain("addresses", domainId)
}
