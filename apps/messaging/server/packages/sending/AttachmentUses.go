package sending

import "mesail.dev/packages/core"

type AttachmentUses struct {
	store core.AttachmentUseStore
}

func NewAttachmentUses(store core.AttachmentUseStore) AttachmentUses {
	return AttachmentUses{store: store}
}

func (uses AttachmentUses) write(message core.Message) error {
	for _, attachmentId := range message.AttachmentIds {
		_, err := uses.store.Insert(uses.use(message, attachmentId))
		if err != nil {
			return Failures{}.store(err)
		}
	}
	return nil
}

func (AttachmentUses) use(message core.Message, attachmentId string) core.AttachmentUse {
	return core.AttachmentUse{
		DomainId:       message.DomainId,
		AttachmentId:   attachmentId,
		ConversationId: message.ConversationId,
		MessageId:      core.MessageIds{}.Wire(message.Month, message.Id),
	}
}
