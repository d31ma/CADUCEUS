package channels

import "mesail.dev/packages/collections"

type Names struct{}

func (Names) Channels(domainId string) string {
	return collections.CollectionName{}.ForDomain("channels", domainId)
}

func (Names) Memberships(domainId string) string {
	return collections.CollectionName{}.ForDomain("channel-members", domainId)
}

func (Names) Registry() string {
	return "outbox-collections"
}
