package sending

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type ThreadRoot struct {
	messages core.MessageStore
}

func NewThreadRoot(messages core.MessageStore) ThreadRoot {
	return ThreadRoot{messages: messages}
}

func (thread ThreadRoot) Find(caller oidc.Caller, conversationId string, threadRootId string) (Thread, error) {
	return thread.route(threadRootId)(caller, conversationId, threadRootId)
}

func (thread ThreadRoot) route(threadRootId string) func(oidc.Caller, string, string) (Thread, error) {
	if threadRootId == "" {
		return thread.none
	}
	return thread.lookup
}

func (ThreadRoot) none(caller oidc.Caller, conversationId string, threadRootId string) (Thread, error) {
	return Thread{}, nil
}

func (thread ThreadRoot) lookup(caller oidc.Caller, conversationId string, targetId string) (Thread, error) {
	target, err := thread.load(caller.DomainId, targetId)
	if err != nil {
		return Thread{}, err
	}
	err = thread.conversationRefusal(target, conversationId)
	if err != nil {
		return Thread{}, err
	}
	root, err := thread.load(caller.DomainId, thread.rootIdOf(target))
	if err != nil {
		return Thread{}, err
	}
	return Thread{Root: root, RootId: core.MessageIds{}.Wire(root.Month, root.Id)}, nil
}

func (thread ThreadRoot) load(domainId string, messageId string) (core.Message, error) {
	month, storedId, err := core.MessageIds{}.Parse(messageId)
	if err != nil {
		return core.Message{}, err
	}
	message, found, err := thread.messages.Find(domainId, month, storedId)
	if err != nil {
		return core.Message{}, Failures{}.store(err)
	}
	return message, thread.missing(found)
}

func (ThreadRoot) missing(found bool) error {
	if found {
		return nil
	}
	return Failures{}.missingMessage()
}

func (ThreadRoot) conversationRefusal(target core.Message, conversationId string) error {
	if target.ConversationId == conversationId {
		return nil
	}
	return Failures{}.missingMessage()
}

func (ThreadRoot) rootIdOf(target core.Message) string {
	if target.ThreadRootId != "" {
		return target.ThreadRootId
	}
	return core.MessageIds{}.Wire(target.Month, target.Id)
}
