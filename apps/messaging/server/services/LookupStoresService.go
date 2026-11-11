import "mesail.dev/packages/core"

// @Service
type LookupStoresService struct {
	// @Needs
	threadKeys *ThreadKeyStoreService
	// @Needs
	mentions *MentionStoreService
	// @Needs
	members *MemberStoreService
	// @Needs
	attachmentUses *AttachmentUseStoreService
}

func (group *LookupStoresService) Into(stores core.Stores) core.Stores {
	stores.ThreadKeys = group.threadKeys
	stores.Mentions = group.mentions
	stores.Members = group.members
	stores.AttachmentUses = group.attachmentUses
	return stores
}
