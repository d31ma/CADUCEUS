package core

import "fmt"

type MemoryAttachmentUses struct {
	rows []AttachmentUse
}

func NewMemoryAttachmentUses() *MemoryAttachmentUses {
	return &MemoryAttachmentUses{}
}

func (store *MemoryAttachmentUses) ByAttachment(domainId string, attachmentId string) ([]AttachmentUse, error) {
	kept := []AttachmentUse{}
	for _, row := range store.rows {
		kept = append(kept, store.ifMatching(row, row.DomainId == domainId && row.AttachmentId == attachmentId)...)
	}
	return kept, nil
}

func (store *MemoryAttachmentUses) Insert(row AttachmentUse) (string, error) {
	row.Id = fmt.Sprintf("%012d", len(store.rows)+1)
	store.rows = append(store.rows, row)
	return row.Id, nil
}

func (store *MemoryAttachmentUses) ifMatching(row AttachmentUse, matched bool) []AttachmentUse {
	if matched {
		return []AttachmentUse{row}
	}
	return nil
}

func (store *MemoryAttachmentUses) All() []AttachmentUse {
	return append([]AttachmentUse{}, store.rows...)
}
