package inbound

import "mesail.dev/packages/core"

type Participants struct{}

func (participants Participants) OfMember(all []core.Participant, memberId string) []core.Participant {
	return participants.ofMember(all, memberId)
}

func (participants Participants) MemberIds(all []core.Participant) []string {
	return participants.memberIds(all)
}

func (participants Participants) ofMember(all []core.Participant, memberId string) []core.Participant {
	kept := []core.Participant{}
	for _, participant := range all {
		kept = append(kept, participants.ifMember(participant, memberId)...)
	}
	return kept
}

func (Participants) ifMember(participant core.Participant, memberId string) []core.Participant {
	if participant.MemberId != memberId {
		return nil
	}
	return []core.Participant{participant}
}

func (participants Participants) memberIds(all []core.Participant) []string {
	memberIds := []string{}
	for _, participant := range all {
		memberIds = append(memberIds, participants.memberIdOf(participant)...)
	}
	return memberIds
}

func (Participants) memberIdOf(participant core.Participant) []string {
	if participant.Kind != (core.ParticipantKinds{}).Member() {
		return nil
	}
	return []string{participant.MemberId}
}
