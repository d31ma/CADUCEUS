package devicebox

import "mesail.dev/packages/collections"

type Names struct{}

func (Names) Slips(domainId string) string {
	return collections.CollectionName{}.ForDomain("devicebox-slips", domainId)
}

func (Names) Bodies(domainId string) string {
	return collections.CollectionName{}.ForDomain("devicebox-bodies", domainId)
}

func (Names) Counts(domainId string) string {
	return collections.CollectionName{}.ForDomain("devicebox-counts", domainId)
}

func (Names) Registry() string {
	return "outbox-collections"
}
