import "mesail.dev/packages/collections"

// @Service
type CollectionNamesService struct {
}

func (service *CollectionNamesService) InboundRaw(domainId string) string {
	return collections.CollectionName{}.ForDomain("inbound-raw", domainId)
}

func (service *CollectionNamesService) Registry() string {
	return "outbox-collections"
}
