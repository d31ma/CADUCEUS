package realtime

import "time"

type PresenceKeeper struct {
	Book        PresenceBook
	Connections ConnectionBook
	FanOut      FanOut
	Now         func() time.Time
}

func (keeper PresenceKeeper) Set(domainId string, memberId string, request PresenceRequest) error {
	return nil
}

func (keeper PresenceKeeper) Connected(domainId string, memberId string) error {
	return nil
}

func (keeper PresenceKeeper) Disconnected(domainId string, memberId string) error {
	return nil
}

func (keeper PresenceKeeper) Query(domainId string, memberIds []string) ([]Presence, error) {
	return []Presence{}, nil
}

func (keeper PresenceKeeper) Settle(domainId string, now string, budget int) (SweepReport, error) {
	return SweepReport{}, nil
}
