package reading

import "mesail.dev/packages/core"

type memberParticipants struct {
	storage storage
}

func (members memberParticipants) of(domainId string, conversationId string) ([]string, error) {
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
