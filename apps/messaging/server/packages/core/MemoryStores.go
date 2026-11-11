package core

type MemoryStores struct{}

func (MemoryStores) New() Stores {
	return Stores{
		Conversations:  NewMemoryConversations(),
		Participants:   NewMemoryParticipants(),
		Messages:       NewMemoryMessages(),
		Reactions:      NewMemoryReactions(),
		ThreadKeys:     NewMemoryThreadKeys(),
		Mentions:       NewMemoryMentions(),
		Members:        NewMemoryMembers(),
		AttachmentUses: NewMemoryAttachmentUses(),
	}
}
