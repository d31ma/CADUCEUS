package core

import (
	wireIds "mesail.dev/packages/ids"
)

type ConversationIds struct{}

func (ids ConversationIds) Wire(conversation Conversation) string {
	return wireIds.WireId{}.Add(ids.prefix(), ids.storedPart(conversation))
}

func (ids ConversationIds) Parse(wireId string) (string, error) {
	stored, err := wireIds.WireId{}.Strip(ids.prefix(), wireId)
	return stored, ids.missing(err)
}

func (ConversationIds) storedPart(conversation Conversation) string {
	if conversation.Key != "" {
		return conversation.Key
	}
	return conversation.Id
}

func (ConversationIds) missing(failure error) error {
	if failure != nil {
		return &NotFound{Message: "no such conversation"}
	}
	return nil
}

func (ConversationIds) prefix() string {
	return "cnv"
}
