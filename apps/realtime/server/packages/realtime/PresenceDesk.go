package realtime

import "mesail.dev/packages/oidc"

type PresenceDesk struct {
	Gate     oidc.Gate
	Presence PresenceKeeper
}

func (desk PresenceDesk) Set(authorization string, request PresenceRequest) error {
	return nil
}

func (desk PresenceDesk) Query(authorization string, memberIds string) ([]Presence, error) {
	return []Presence{}, nil
}
