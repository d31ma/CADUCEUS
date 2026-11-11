import "mesail.dev/packages/core"

type MessageQuery struct {
	ConversationId string
	ThreadRootId   string
	Page           core.PageRequest
}
