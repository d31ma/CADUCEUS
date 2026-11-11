package search

import "mesail.dev/packages/collections"

type Collections struct{}

func (Collections) Documents(domainId string, month string) string {
	return collections.CollectionName{}.ForDomainMonth("search-index", domainId, month)
}

func (Collections) Months(domainId string) string {
	return collections.CollectionName{}.ForDomain("search-months", domainId)
}

func (Collections) Locations(domainId string) string {
	return collections.CollectionName{}.ForDomain("search-locations", domainId)
}

func (Collections) SeenEvents(domainId string) string {
	return collections.CollectionName{}.ForDomain("events-seen", domainId)
}
