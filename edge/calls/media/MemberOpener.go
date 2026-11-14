package media

import (
	"time"

	"mesail.dev/edge/calls/admission"
)

type MemberOpener struct {
	factory     PeerFactory
	clock       admission.Clock
	connectWait time.Duration
}

func NewMemberOpener(factory PeerFactory, clock admission.Clock, connectWait time.Duration) MemberOpener {
	return MemberOpener{factory: factory, clock: clock, connectWait: connectWait}
}

func (opener MemberOpener) Open(seat Seat, offer string, host MemberHost) (*Member, error) {
	parts, err := opener.factory.New()
	if err != nil {
		return nil, err
	}
	member := NewMember(seat, parts, host, opener.clock)
	return member, opener.negotiated(member, offer)
}

func (opener MemberOpener) negotiated(member *Member, offer string) error {
	err := member.Negotiate(offer, opener.connectWait)
	if err != nil {
		member.Close()
	}
	return err
}
