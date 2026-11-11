import "mesail.dev/packages/core"

// @Service
type AttachmentUseStoreService struct {
	// @Needs
	tenant *TenantCollectionsService
	// @Needs
	names *CollectionNamesService
}

func (store *AttachmentUseStoreService) Insert(use core.AttachmentUse) (string, error) {
	collection := store.names.AttachmentUses(use.DomainId)
	return store.tenant.Insert(collection, use)
}

func (store *AttachmentUseStoreService) ByAttachment(domainId string, attachmentId string) ([]core.AttachmentUse, error) {
	uses := []core.AttachmentUse{}
	failure := store.tenant.Matching(store.names.AttachmentUses(domainId), domainId, store.matching(attachmentId), &uses)
	return uses, failure
}

func (store *AttachmentUseStoreService) matching(attachmentId string) map[string]any {
	return map[string]any{store.attachmentField(): attachmentId}
}

func (store *AttachmentUseStoreService) attachmentField() string {
	return "attachmentId"
}
