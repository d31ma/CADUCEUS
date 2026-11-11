package inbound

import "mesail.dev/packages/core"

type EmailResume struct {
	stores core.Stores
	ports  core.Ports
}

func NewEmailResume(stores core.Stores, ports core.Ports) EmailResume {
	return EmailResume{stores: stores, ports: ports}
}

func (resume EmailResume) Continue(existing core.Message) error {
	return resume.finishUnlessStaged(existing)
}

func (resume EmailResume) finishUnlessStaged(existing core.Message) error {
	if existing.EventsStaged {
		return nil
	}
	return resume.finish(existing)
}

func (resume EmailResume) finish(existing core.Message) error {
	conversation, err := NewLookups(resume.stores).ExistingConversation(existing.DomainId, existing.ConversationId)
	if err != nil {
		return err
	}
	return NewMessageFinisher(resume.stores, resume.ports).Finish(Placement{Conversation: conversation}, existing)
}
