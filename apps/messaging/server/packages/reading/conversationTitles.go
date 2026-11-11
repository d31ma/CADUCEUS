package reading

import (
	"strings"

	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type conversationTitles struct {
	storage storage
}

func (titles conversationTitles) of(caller oidc.Caller, conversation core.Conversation) (string, error) {
	participants, err := titles.participantsFor(conversation)
	names := titles.names(titles.others(participants, caller.MemberId))
	direct := titles.direct(conversation, strings.Join(names, titles.separator()))
	return titles.chosen(conversation, direct), err
}

func (titles conversationTitles) participantsFor(conversation core.Conversation) ([]core.Participant, error) {
	if conversation.Kind == (core.ConversationKinds{}).Channel() {
		return []core.Participant{}, nil
	}
	return titles.storage.participantsIn(conversation.DomainId, core.ConversationIds{}.Wire(conversation))
}

func (titles conversationTitles) others(participants []core.Participant, memberId string) []core.Participant {
	others := []core.Participant{}
	for _, participant := range participants {
		others = append(others, titles.ifOther(participant, memberId)...)
	}
	return others
}

func (conversationTitles) ifOther(participant core.Participant, memberId string) []core.Participant {
	if participant.MemberId != "" && participant.MemberId == memberId {
		return nil
	}
	return []core.Participant{participant}
}

func (titles conversationTitles) names(participants []core.Participant) []string {
	names := []string{}
	for _, participant := range participants {
		names = append(names, titles.nameOf(participant))
	}
	return names
}

func (conversationTitles) nameOf(participant core.Participant) string {
	if participant.DisplayName != "" {
		return participant.DisplayName
	}
	return participant.Address
}

func (conversationTitles) direct(conversation core.Conversation, names string) string {
	if conversation.HasExternal && conversation.Subject != "" {
		return conversation.Subject
	}
	return names
}

func (titles conversationTitles) chosen(conversation core.Conversation, direct string) string {
	if conversation.Kind == (core.ConversationKinds{}).Channel() {
		return titles.channelMark() + conversation.ChannelName
	}
	return direct
}

func (conversationTitles) separator() string {
	return ", "
}

func (conversationTitles) channelMark() string {
	return "#"
}
