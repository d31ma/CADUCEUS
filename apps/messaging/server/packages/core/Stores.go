package core

type Stores struct {
	Conversations  ConversationStore
	Participants   ParticipantStore
	Messages       MessageStore
	Reactions      ReactionStore
	ThreadKeys     ThreadKeyStore
	Mentions       MentionStore
	Members        MemberStore
	AttachmentUses AttachmentUseStore
}
