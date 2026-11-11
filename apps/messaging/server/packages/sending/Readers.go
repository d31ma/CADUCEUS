package sending

import "mesail.dev/packages/core"

type Readers struct{}

func (readers Readers) of(participants []core.Participant) []string {
	memberIds := []string{}
	for _, participant := range participants {
		memberIds = append(memberIds, readers.ifMember(participant)...)
	}
	return memberIds
}

func (Readers) ifMember(participant core.Participant) []string {
	if participant.Kind != (core.ParticipantKinds{}).Member() {
		return nil
	}
	return []string{participant.MemberId}
}
