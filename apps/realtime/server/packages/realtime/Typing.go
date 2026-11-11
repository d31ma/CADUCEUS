package realtime

import "slices"

type Typing struct {
	Members MemberBook
	FanOut  FanOut
}

func (typing Typing) Typing(connection Connection, conversationId string) error {
	err := typing.named(conversationId)
	if err != nil {
		return err
	}
	memberIds, err := typing.Members.Members(connection.DomainId, conversationId)
	if err != nil {
		return err
	}
	err = typing.member(memberIds, connection.MemberId)
	if err != nil {
		return err
	}
	event := Event{Type: "typing", ConversationId: conversationId, MemberId: connection.MemberId}
	return typing.FanOut.ToMembersExcept(connection.DomainId, memberIds, connection.MemberId, event)
}

func (Typing) named(conversationId string) error {
	if conversationId == "" {
		return Invalid{Field: "conversationId", Reason: "a conversation id is required"}
	}
	return nil
}

func (Typing) member(memberIds []string, memberId string) error {
	if !slices.Contains(memberIds, memberId) {
		return Missing{What: "conversation"}
	}
	return nil
}
