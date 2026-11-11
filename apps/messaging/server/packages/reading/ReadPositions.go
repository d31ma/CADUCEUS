package reading

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
)

type ReadPositions struct {
	access   core.Access
	messages conversationMessages
	storage  storage
	ports    core.Ports
}

func NewReadPositions(stores core.Stores, ports core.Ports) ReadPositions {
	access := core.NewAccess(stores)
	return ReadPositions{access: access, messages: conversationMessages{access: access}, storage: storage{stores: stores}, ports: ports}
}

func (positions ReadPositions) Move(caller oidc.Caller, conversationId string, messageId string) error {
	conversation, err := positions.access.Conversation(caller, conversationId)
	if err != nil {
		return err
	}
	message, err := positions.messages.in(caller, conversation, messageId)
	if err != nil {
		return err
	}
	participant, joined, err := positions.access.Participant(caller, conversation)
	if err != nil {
		return err
	}
	return positions.forward(participant, joined, message)
}

func (positions ReadPositions) forward(participant core.Participant, joined bool, message core.Message) error {
	if !joined || message.Sequence <= participant.ReadSequence {
		return nil
	}
	return positions.recorded(positions.moved(participant, message))
}

func (positions ReadPositions) moved(participant core.Participant, message core.Message) core.Participant {
	participant.ReadSequence = message.Sequence
	participant.ReadMessageId = core.MessageIds{}.Wire(message.Month, message.Id)
	participant.ReadAt = clock{now: positions.ports.Now}.stamp()
	return participant
}

func (positions ReadPositions) recorded(participant core.Participant) error {
	payload := events.ReadPositionMoved{
		ConversationId: participant.ConversationId,
		MemberId:       participant.MemberId,
		MessageId:      participant.ReadMessageId,
		ReadAt:         participant.ReadAt,
	}
	staged, err := positions.ports.Stager.Stage(participant.OutboxFields, events.Types{}.ReadPositionMoved(), participant.DomainId, payload)
	if err != nil {
		return err
	}
	participant.OutboxFields = staged
	return positions.storage.saveParticipant(participant)
}
