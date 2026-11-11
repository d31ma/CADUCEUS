package reading

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type threadRoots struct {
	messages conversationMessages
}

func (roots threadRoots) check(caller oidc.Caller, conversation core.Conversation, threadRootId string) error {
	if threadRootId == "" {
		return nil
	}
	_, err := roots.messages.in(caller, conversation, threadRootId)
	return err
}
