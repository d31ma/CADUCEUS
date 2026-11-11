package realtime

import (
	"errors"
	"slices"
	"time"
)

type PresenceRequestCheck struct {
	Now time.Time
}

func (check PresenceRequestCheck) Check(request PresenceRequest) error {
	return errors.Join(check.state(request.State), check.end(request))
}

func (PresenceRequestCheck) state(state string) error {
	if !slices.Contains(PresenceStates{}.Online(), state) {
		return Invalid{Field: "state", Reason: "must be active, idle or dnd"}
	}
	return nil
}

func (check PresenceRequestCheck) end(request PresenceRequest) error {
	until := TimeText{}.At(request.DoNotDisturbUntil)
	if request.State == (PresenceStates{}).DoNotDisturb() && !until.After(check.Now) {
		return Invalid{Field: "doNotDisturbUntil", Reason: "must be an RFC 3339 time in the future"}
	}
	return nil
}
