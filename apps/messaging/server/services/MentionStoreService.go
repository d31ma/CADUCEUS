import "mesail.dev/packages/core"

// @Service
type MentionStoreService struct {
	// @Needs
	tenant *TenantCollectionsService
	// @Needs
	names *CollectionNamesService
}

func (store *MentionStoreService) Insert(mention core.Mention) (string, error) {
	return store.tenant.Insert(store.names.Mentions(mention.DomainId), mention)
}

func (store *MentionStoreService) OfMember(domainId string, memberId string) ([]core.Mention, error) {
	mentions := []core.Mention{}
	failure := store.tenant.Matching(store.names.Mentions(domainId), domainId, map[string]any{store.memberField(): memberId}, &mentions)
	return mentions, failure
}

func (store *MentionStoreService) memberField() string {
	return "memberId"
}
