package backups

import "mesail.dev/packages/events"

type Removal struct {
	Desk Desk
}

func (removal Removal) Handle(envelope events.Envelope) error {
	member, err := removal.member(envelope)
	if err != nil {
		return err
	}
	return removal.removeWhenGone(envelope.DomainId, member)
}

func (Removal) member(envelope events.Envelope) (events.MemberUpserted, error) {
	member := events.MemberUpserted{}
	err := events.PayloadReader{}.Read(envelope, &member)
	if err != nil {
		return member, events.InvalidEnvelope{Reason: "payload: " + err.Error()}
	}
	return member, nil
}

func (removal Removal) removeWhenGone(domainId string, member events.MemberUpserted) error {
	if member.State != removal.removedState() {
		return nil
	}
	return removal.Desk.Remove(domainId, member.MemberId)
}

func (Removal) removedState() string {
	return "removed"
}
