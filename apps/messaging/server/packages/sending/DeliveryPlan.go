package sending

import (
	"slices"

	"mesail.dev/packages/core"
	kit "mesail.dev/packages/recipients"
)

type DeliveryPlan struct{}

func (plan DeliveryPlan) rows(placement Placement, chosen []Recipient, sendToExternal bool) []core.DeliveryRecord {
	covered := plan.covered(placement.Sender, chosen)
	return slices.Concat(plan.explicit(chosen), plan.implied(placement, covered, sendToExternal))
}

func (DeliveryPlan) covered(sender core.Participant, chosen []Recipient) map[string]bool {
	covered := map[string]bool{Identities{}.ofParticipant(sender): true}
	for _, recipient := range chosen {
		covered[Identities{}.ofRecipient(recipient)] = true
	}
	return covered
}

func (plan DeliveryPlan) explicit(chosen []Recipient) []core.DeliveryRecord {
	rows := []core.DeliveryRecord{}
	for _, recipient := range chosen {
		rows = append(rows, plan.recipientRows(recipient)...)
	}
	return rows
}

func (plan DeliveryPlan) recipientRows(recipient Recipient) []core.DeliveryRecord {
	deliveries := kit.Deliveries{}
	chat := plan.onlyIf(recipient.Delivery != string(deliveries.Email()), plan.chatRow(recipient.Address, recipient.MemberId, recipient.Role))
	email := plan.onlyIf(recipient.Delivery != string(deliveries.Chat()), plan.emailRow(recipient.Address, recipient.MemberId, recipient.Role))
	return slices.Concat(chat, email)
}

func (plan DeliveryPlan) implied(placement Placement, covered map[string]bool, sendToExternal bool) []core.DeliveryRecord {
	rows := []core.DeliveryRecord{}
	for _, participant := range plan.uncovered(placement.Existing, covered) {
		rows = append(rows, plan.participantRows(participant, placement.Conversation.Kind, sendToExternal)...)
	}
	return rows
}

func (plan DeliveryPlan) uncovered(existing []core.Participant, covered map[string]bool) []core.Participant {
	kept := []core.Participant{}
	for _, participant := range existing {
		kept = append(kept, plan.ifUncovered(participant, covered)...)
	}
	return kept
}

func (DeliveryPlan) ifUncovered(participant core.Participant, covered map[string]bool) []core.Participant {
	if covered[Identities{}.ofParticipant(participant)] {
		return nil
	}
	return []core.Participant{participant}
}

func (plan DeliveryPlan) participantRows(participant core.Participant, conversationKind string, sendToExternal bool) []core.DeliveryRecord {
	chat := plan.onlyIf(plan.chatWanted(participant, conversationKind), plan.chatRow(participant.Address, participant.MemberId, participant.Role))
	email := plan.onlyIf(plan.emailWanted(participant, conversationKind, sendToExternal), plan.emailRow(participant.Address, participant.MemberId, participant.Role))
	return slices.Concat(chat, email)
}

func (DeliveryPlan) chatWanted(participant core.Participant, conversationKind string) bool {
	return participant.Kind == (core.ParticipantKinds{}).Member() && conversationKind == (core.ConversationKinds{}).Direct()
}

func (DeliveryPlan) emailWanted(participant core.Participant, conversationKind string, sendToExternal bool) bool {
	return participant.Kind != (core.ParticipantKinds{}).Member() && (conversationKind == (core.ConversationKinds{}).Direct() || sendToExternal)
}

func (DeliveryPlan) onlyIf(wanted bool, row core.DeliveryRecord) []core.DeliveryRecord {
	if wanted {
		return []core.DeliveryRecord{row}
	}
	return nil
}

func (DeliveryPlan) chatRow(address string, memberId string, role string) core.DeliveryRecord {
	return core.DeliveryRecord{Recipient: address, MemberId: memberId, Channel: core.DeliveryChannels{}.Chat(), Role: role, Status: core.DeliveryStatuses{}.Delivered()}
}

func (DeliveryPlan) emailRow(address string, memberId string, role string) core.DeliveryRecord {
	return core.DeliveryRecord{Recipient: address, MemberId: memberId, Channel: core.DeliveryChannels{}.Email(), Role: role, Status: core.DeliveryStatuses{}.Queued()}
}
