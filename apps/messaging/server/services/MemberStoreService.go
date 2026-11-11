import "mesail.dev/packages/core"

// @Service
type MemberStoreService struct {
	// @Needs
	tenant *TenantCollectionsService
	// @Needs
	names *CollectionNamesService
}

func (store *MemberStoreService) Save(member core.Member) error {
	existing, found, failure := store.ByMemberId(member.DomainId, member.MemberId)
	return store.write(failure, member, existing, found)
}

func (store *MemberStoreService) ByMemberId(domainId string, memberId string) (core.Member, bool, error) {
	return store.lookup(domainId, store.memberField(), memberId)
}

func (store *MemberStoreService) ByLocalPart(domainId string, localPart string) (core.Member, bool, error) {
	return store.lookup(domainId, store.localPartField(), localPart)
}

func (store *MemberStoreService) lookup(domainId string, field string, value string) (core.Member, bool, error) {
	member := core.Member{}
	found, failure := store.tenant.First(store.names.Members(domainId), domainId, map[string]any{field: value}, &member)
	return member, found, failure
}

func (store *MemberStoreService) write(failure error, member core.Member, existing core.Member, found bool) error {
	switch {
	case failure != nil:
		return failure
	case found:
		return store.tenant.Replace(store.names.Members(member.DomainId), existing.Id, member)
	default:
		_, insertFailure := store.tenant.Insert(store.names.Members(member.DomainId), member)
		return insertFailure
	}
}

func (store *MemberStoreService) memberField() string {
	return "memberId"
}

func (store *MemberStoreService) localPartField() string {
	return "localPart"
}
