package reading

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

type memberParticipants struct {
	storage storage
}

func (members memberParticipants) of(domainId string, conversationId string) ([]string, error) {
	conversation, err := members.conversation(domainId, conversationId)
	if err != nil {
		return nil, err
	}
	return members.source(conversation.Kind)(domainId, conversationId)
}

func (members memberParticipants) conversation(domainId string, conversationId string) (core.Conversation, error) {
	storedPart, err := core.ConversationIds{}.Parse(conversationId)
	if err != nil {
		return core.Conversation{}, err
	}
	conversation, _, err := members.storage.conversation(domainId, storedPart)
	return conversation, err
}

func (members memberParticipants) source(conversationKind string) func(string, string) ([]string, error) {
	if conversationKind == (events.ReaderLists{}).ChannelKind() {
		return members.none
	}
	return members.listed
}

func (memberParticipants) none(domainId string, conversationId string) ([]string, error) {
	return []string{}, nil
}

func (members memberParticipants) listed(domainId string, conversationId string) ([]string, error) {
	rows, err := members.storage.participantsIn(domainId, conversationId)
	return members.memberIds(rows), err
}

func (members memberParticipants) memberIds(rows []core.Participant) []string {
	memberIds := []string{}
	for _, row := range rows {
		memberIds = append(memberIds, members.ifMember(row)...)
	}
	return memberIds
}

func (memberParticipants) ifMember(row core.Participant) []string {
	if row.Kind == (core.ParticipantKinds{}).Member() && row.MemberId != "" {
		return []string{row.MemberId}
	}
	return nil
}
