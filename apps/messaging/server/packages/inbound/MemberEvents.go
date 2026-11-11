package inbound

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

type MemberEvents struct {
	members core.MemberStore
}

func NewMemberEvents(members core.MemberStore) MemberEvents {
	return MemberEvents{members: members}
}

func (memberEvents MemberEvents) Upserted(envelope events.Envelope) error {
	payload := events.MemberUpserted{}
	err := events.PayloadReader{}.Read(envelope, &payload)
	if err != nil {
		return err
	}
	err = memberEvents.members.Save(memberEvents.member(envelope.DomainId, payload))
	if err != nil {
		return Unavailability{}.Of(err)
	}
	return nil
}

func (MemberEvents) member(domainId string, payload events.MemberUpserted) core.Member {
	return core.Member{
		DomainId:       domainId,
		MemberId:       payload.MemberId,
		DisplayName:    payload.DisplayName,
		PrimaryAddress: payload.PrimaryAddress,
		LocalPart:      Addresses{}.LocalPart(payload.PrimaryAddress),
		State:          payload.State,
	}
}
