import "mesail.dev/packages/collections"

// @Service
type CollectionNamesService struct{}

func (names *CollectionNamesService) Notifications(domainId string, yearMonth string) string {
	return collections.CollectionName{}.ForDomainMonth("notifications", domainId, yearMonth)
}

func (names *CollectionNamesService) PushSubscriptions(domainId string) string {
	return collections.CollectionName{}.ForDomain("push-subscriptions", domainId)
}

func (names *CollectionNamesService) MemberSettings(domainId string) string {
	return collections.CollectionName{}.ForDomain("notify-settings", domainId)
}

func (names *CollectionNamesService) Threads(domainId string) string {
	return collections.CollectionName{}.ForDomain("notify-threads", domainId)
}

func (names *CollectionNamesService) Summaries(domainId string) string {
	return collections.CollectionName{}.ForDomain("notify-summaries", domainId)
}

func (names *CollectionNamesService) Fanout(domainId string) string {
	return collections.CollectionName{}.ForDomain("notify-fanout", domainId)
}

func (names *CollectionNamesService) SeenEvents(domainId string) string {
	return collections.CollectionName{}.ForDomain("events-seen", domainId)
}

func (names *CollectionNamesService) Domains() string {
	return "notify-domains"
}

func (names *CollectionNamesService) OutboxCollections() string {
	return "outbox-collections"
}
