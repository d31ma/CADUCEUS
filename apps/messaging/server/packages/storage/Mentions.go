package storage

import "mesail.dev/packages/core"

type Mentions struct {
	Tenant Tenant
	Names  CollectionNames
}

func (store Mentions) Insert(mention core.Mention) (string, error) {
	return store.Tenant.Insert(store.Names.Mentions(mention.DomainId), mention)
}

func (store Mentions) OfMember(domainId string, memberId string) ([]core.Mention, error) {
	mentions := []core.Mention{}
	failure := store.Tenant.Matching(store.Names.Mentions(domainId), domainId, map[string]any{store.memberField(): memberId}, &mentions)
	return mentions, failure
}

func (Mentions) memberField() string {
	return "memberId"
}
