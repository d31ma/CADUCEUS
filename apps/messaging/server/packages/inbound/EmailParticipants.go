package inbound

import "mesail.dev/packages/core"

type EmailParticipants struct{}

func (participants EmailParticipants) Sender(email ReceivedEmail, conversationId string) core.Participant {
	return core.Participant{
		DomainId:       email.DomainId,
		ConversationId: conversationId,
		Kind:           participants.senderKind(email.Mail.AuthResults.Handoff),
		Address:        Addresses{}.Normal(email.Mail.Headers.From),
		Role:           core.RecipientRoles{}.To(),
	}
}

func (EmailParticipants) Member(domainId string, conversationId string, entry core.AddressEntry, member core.Member) core.Participant {
	return core.Participant{
		DomainId:       domainId,
		ConversationId: conversationId,
		Kind:           core.ParticipantKinds{}.Member(),
		MemberId:       entry.TargetId,
		Address:        Addresses{}.Normal(entry.Address),
		DisplayName:    member.DisplayName,
		Role:           core.RecipientRoles{}.To(),
	}
}

func (participants EmailParticipants) Direct(email ReceivedEmail, conversationId string, members []core.Participant) []core.Participant {
	wanted := append([]core.Participant{participants.Sender(email, conversationId)}, members...)
	return append(wanted, participants.outside(email, conversationId)...)
}

func (EmailParticipants) senderKind(handoff bool) string {
	if handoff {
		return core.ParticipantKinds{}.OtherDomain()
	}
	return core.ParticipantKinds{}.External()
}

func (participants EmailParticipants) outside(email ReceivedEmail, conversationId string) []core.Participant {
	outside := []core.Participant{}
	headers := email.Mail.Headers
	for _, address := range append(append([]string{}, headers.To...), headers.Cc...) {
		outside = append(outside, participants.ifOutside(email, conversationId, address)...)
	}
	return outside
}

func (EmailParticipants) ifOutside(email ReceivedEmail, conversationId string, address string) []core.Participant {
	addresses := Addresses{}
	if addresses.Domain(address) == addresses.Normal(email.DomainId) {
		return nil
	}
	return []core.Participant{{
		DomainId:       email.DomainId,
		ConversationId: conversationId,
		Kind:           core.ParticipantKinds{}.External(),
		Address:        addresses.Normal(address),
		Role:           core.RecipientRoles{}.Cc(),
	}}
}
