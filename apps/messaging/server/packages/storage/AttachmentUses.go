package storage

import "mesail.dev/packages/core"

type AttachmentUses struct {
	Tenant Tenant
	Names  CollectionNames
}

func (store AttachmentUses) Insert(use core.AttachmentUse) (string, error) {
	return store.Tenant.Insert(store.Names.AttachmentUses(use.DomainId), use)
}

func (store AttachmentUses) ByAttachment(domainId string, attachmentId string) ([]core.AttachmentUse, error) {
	uses := []core.AttachmentUse{}
	failure := store.Tenant.Matching(store.Names.AttachmentUses(domainId), domainId, map[string]any{store.attachmentField(): attachmentId}, &uses)
	return uses, failure
}

func (AttachmentUses) attachmentField() string {
	return "attachmentId"
}
