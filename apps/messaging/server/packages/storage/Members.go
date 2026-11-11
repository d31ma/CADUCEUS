package storage

import "mesail.dev/packages/core"

type Members struct {
	Tenant Tenant
	Names  CollectionNames
}

func (store Members) Save(member core.Member) error {
	existing, found, failure := store.ByMemberId(member.DomainId, member.MemberId)
	return store.write(failure, member, existing, found)
}

func (store Members) ByMemberId(domainId string, memberId string) (core.Member, bool, error) {
	return store.lookup(domainId, store.memberField(), memberId)
}

func (store Members) ByLocalPart(domainId string, localPart string) (core.Member, bool, error) {
	return store.lookup(domainId, store.localPartField(), localPart)
}

func (store Members) lookup(domainId string, field string, value string) (core.Member, bool, error) {
	member := core.Member{}
	found, failure := store.Tenant.First(store.Names.Members(domainId), domainId, map[string]any{field: value}, &member)
	return member, found, failure
}

func (store Members) write(failure error, member core.Member, existing core.Member, found bool) error {
	switch {
	case failure != nil:
		return failure
	case found:
		return store.Tenant.Replace(store.Names.Members(member.DomainId), existing.Id, member)
	default:
		_, insertFailure := store.Tenant.Insert(store.Names.Members(member.DomainId), member)
		return insertFailure
	}
}

func (Members) memberField() string {
	return "memberId"
}

func (Members) localPartField() string {
	return "localPart"
}
