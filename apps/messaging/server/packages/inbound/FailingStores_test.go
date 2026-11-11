package inbound

import (
	"errors"

	"mesail.dev/packages/core"
)

type Gate struct {
	calls  int
	failAt int
}

func (gate *Gate) check() error {
	gate.calls++
	if gate.calls == gate.failAt {
		return errors.New("the disk is gone")
	}
	return nil
}

type FailingConversations struct {
	inner core.ConversationStore
	gate  *Gate
}

func (store FailingConversations) Insert(row core.Conversation) (string, error) {
	if err := store.gate.check(); err != nil {
		return "", err
	}
	return store.inner.Insert(row)
}

func (store FailingConversations) Save(row core.Conversation) error {
	if err := store.gate.check(); err != nil {
		return err
	}
	return store.inner.Save(row)
}

func (store FailingConversations) Find(domainId string, storedPart string) (core.Conversation, bool, error) {
	if err := store.gate.check(); err != nil {
		return core.Conversation{}, false, err
	}
	return store.inner.Find(domainId, storedPart)
}

func (store FailingConversations) ByChannel(domainId string, channelId string) (core.Conversation, bool, error) {
	if err := store.gate.check(); err != nil {
		return core.Conversation{}, false, err
	}
	return store.inner.ByChannel(domainId, channelId)
}

func (store FailingConversations) ByParticipantKey(domainId string, key string) (core.Conversation, bool, error) {
	if err := store.gate.check(); err != nil {
		return core.Conversation{}, false, err
	}
	return store.inner.ByParticipantKey(domainId, key)
}

type FailingParticipants struct {
	inner core.ParticipantStore
	gate  *Gate
}

func (store FailingParticipants) Insert(row core.Participant) (string, error) {
	if err := store.gate.check(); err != nil {
		return "", err
	}
	return store.inner.Insert(row)
}

func (store FailingParticipants) Save(row core.Participant) error {
	if err := store.gate.check(); err != nil {
		return err
	}
	return store.inner.Save(row)
}

func (store FailingParticipants) Delete(domainId string, id string) error {
	if err := store.gate.check(); err != nil {
		return err
	}
	return store.inner.Delete(domainId, id)
}

func (store FailingParticipants) OfConversation(domainId string, conversationId string) ([]core.Participant, error) {
	if err := store.gate.check(); err != nil {
		return nil, err
	}
	return store.inner.OfConversation(domainId, conversationId)
}

func (store FailingParticipants) OfMember(domainId string, memberId string) ([]core.Participant, error) {
	if err := store.gate.check(); err != nil {
		return nil, err
	}
	return store.inner.OfMember(domainId, memberId)
}

type FailingMessages struct {
	inner core.MessageStore
	gate  *Gate
}

func (store FailingMessages) Insert(row core.Message) (string, error) {
	if err := store.gate.check(); err != nil {
		return "", err
	}
	return store.inner.Insert(row)
}

func (store FailingMessages) Save(row core.Message) error {
	if err := store.gate.check(); err != nil {
		return err
	}
	return store.inner.Save(row)
}

func (store FailingMessages) Find(domainId string, month string, id string) (core.Message, bool, error) {
	if err := store.gate.check(); err != nil {
		return core.Message{}, false, err
	}
	return store.inner.Find(domainId, month, id)
}

func (store FailingMessages) BySendKey(domainId string, month string, sendKey string) (core.Message, bool, error) {
	if err := store.gate.check(); err != nil {
		return core.Message{}, false, err
	}
	return store.inner.BySendKey(domainId, month, sendKey)
}

func (store FailingMessages) OfConversation(domainId string, month string, conversationId string) ([]core.Message, error) {
	if err := store.gate.check(); err != nil {
		return nil, err
	}
	return store.inner.OfConversation(domainId, month, conversationId)
}

type FailingThreadKeys struct {
	inner core.ThreadKeyStore
	gate  *Gate
}

func (store FailingThreadKeys) Insert(row core.ThreadKey) (string, error) {
	if err := store.gate.check(); err != nil {
		return "", err
	}
	return store.inner.Insert(row)
}

func (store FailingThreadKeys) ByKey(domainId string, key string) (core.ThreadKey, bool, error) {
	if err := store.gate.check(); err != nil {
		return core.ThreadKey{}, false, err
	}
	return store.inner.ByKey(domainId, key)
}

type FailingMembers struct {
	inner core.MemberStore
	gate  *Gate
}

func (store FailingMembers) Save(row core.Member) error {
	if err := store.gate.check(); err != nil {
		return err
	}
	return store.inner.Save(row)
}

func (store FailingMembers) ByMemberId(domainId string, memberId string) (core.Member, bool, error) {
	if err := store.gate.check(); err != nil {
		return core.Member{}, false, err
	}
	return store.inner.ByMemberId(domainId, memberId)
}

func (store FailingMembers) ByLocalPart(domainId string, localPart string) (core.Member, bool, error) {
	if err := store.gate.check(); err != nil {
		return core.Member{}, false, err
	}
	return store.inner.ByLocalPart(domainId, localPart)
}

func FailingAt(stores core.Stores, failAt int) (core.Stores, *Gate) {
	gate := &Gate{failAt: failAt}
	return core.Stores{
		Conversations: FailingConversations{inner: stores.Conversations, gate: gate},
		Participants:  FailingParticipants{inner: stores.Participants, gate: gate},
		Messages:      FailingMessages{inner: stores.Messages, gate: gate},
		Reactions:     stores.Reactions,
		ThreadKeys:    FailingThreadKeys{inner: stores.ThreadKeys, gate: gate},
		Mentions:      stores.Mentions,
		Members:       FailingMembers{inner: stores.Members, gate: gate},
	}, gate
}
