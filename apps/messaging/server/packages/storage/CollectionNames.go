package storage

import "mesail.dev/packages/collections"

type CollectionNames struct{}

func (CollectionNames) Conversations(domainId string) string {
	return collections.CollectionName{}.ForDomain("conversations", domainId)
}

func (CollectionNames) Participants(domainId string) string {
	return collections.CollectionName{}.ForDomain("participants", domainId)
}

func (CollectionNames) Messages(domainId string, month string) string {
	return collections.CollectionName{}.ForDomainMonth("messages", domainId, month)
}

func (CollectionNames) Reactions(domainId string) string {
	return collections.CollectionName{}.ForDomain("reactions", domainId)
}

func (CollectionNames) ThreadKeys(domainId string) string {
	return collections.CollectionName{}.ForDomain("email-threads", domainId)
}

func (CollectionNames) Mentions(domainId string) string {
	return collections.CollectionName{}.ForDomain("mentions", domainId)
}

func (CollectionNames) Members(domainId string) string {
	return collections.CollectionName{}.ForDomain("members", domainId)
}

func (CollectionNames) AttachmentUses(domainId string) string {
	return collections.CollectionName{}.ForDomain("attachment-uses", domainId)
}

func (CollectionNames) Seen(domainId string) string {
	return collections.CollectionName{}.ForDomain("events-seen", domainId)
}

func (CollectionNames) Registry() string {
	return "outbox-collections"
}
