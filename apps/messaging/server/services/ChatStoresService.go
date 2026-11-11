import "mesail.dev/packages/core"

// @Service
type ChatStoresService struct {
	// @Needs
	conversations *ConversationStoreService
	// @Needs
	participants *ParticipantStoreService
	// @Needs
	messages *MessageStoreService
	// @Needs
	reactions *ReactionStoreService
}

func (group *ChatStoresService) Into(stores core.Stores) core.Stores {
	stores.Conversations = group.conversations
	stores.Participants = group.participants
	stores.Messages = group.messages
	stores.Reactions = group.reactions
	return stores
}
