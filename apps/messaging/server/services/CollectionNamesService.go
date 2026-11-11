import "mesail.dev/packages/collections"

// @Service
type CollectionNamesService struct {
}

func (service *CollectionNamesService) Conversations(domainId string) string {
	return collections.CollectionName{}.ForDomain("conversations", domainId)
}

func (service *CollectionNamesService) Participants(domainId string) string {
	return collections.CollectionName{}.ForDomain("participants", domainId)
}

func (service *CollectionNamesService) Messages(domainId string, month string) string {
	return collections.CollectionName{}.ForDomainMonth("messages", domainId, month)
}

func (service *CollectionNamesService) Reactions(domainId string) string {
	return collections.CollectionName{}.ForDomain("reactions", domainId)
}

func (service *CollectionNamesService) ThreadKeys(domainId string) string {
	return collections.CollectionName{}.ForDomain("email-threads", domainId)
}

func (service *CollectionNamesService) Mentions(domainId string) string {
	return collections.CollectionName{}.ForDomain("mentions", domainId)
}

func (service *CollectionNamesService) Members(domainId string) string {
	return collections.CollectionName{}.ForDomain("members", domainId)
}

func (service *CollectionNamesService) AttachmentUses(domainId string) string {
	return collections.CollectionName{}.ForDomain("attachment-uses", domainId)
}

func (service *CollectionNamesService) Seen(domainId string) string {
	return collections.CollectionName{}.ForDomain("events-seen", domainId)
}

func (service *CollectionNamesService) Registry() string {
	return "outbox-collections"
}
