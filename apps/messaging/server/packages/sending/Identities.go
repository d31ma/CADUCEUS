package sending

import "mesail.dev/packages/core"

type Identities struct{}

func (identities Identities) ofParticipant(participant core.Participant) string {
	return identities.of(participant.Kind, participant.MemberId, participant.Address)
}

func (identities Identities) ofRecipient(recipient Recipient) string {
	return identities.of(recipient.Kind, recipient.MemberId, recipient.Address)
}

func (Identities) of(kind string, memberId string, address string) string {
	if kind == (core.ParticipantKinds{}).Member() {
		return "member:" + memberId
	}
	return "address:" + address
}
