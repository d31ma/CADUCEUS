package realtime

import "mesail.dev/packages/oidc"

type PresenceDesk struct {
	Gate     oidc.Gate
	Presence PresenceKeeper
}

func (desk PresenceDesk) Set(authorization string, request PresenceRequest) error {
	caller, err := CallerGate{Gate: desk.Gate}.Caller(authorization)
	if err != nil {
		return err
	}
	return desk.Presence.Set(caller.DomainId, caller.MemberId, request)
}

func (desk PresenceDesk) Query(authorization string, memberIds string) ([]Presence, error) {
	caller, err := CallerGate{Gate: desk.Gate}.Caller(authorization)
	if err != nil {
		return nil, err
	}
	return desk.Presence.Query(caller.DomainId, MemberIdList{}.Parse(memberIds))
}
