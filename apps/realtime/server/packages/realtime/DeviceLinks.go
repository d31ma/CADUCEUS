package realtime

import "time"

type DeviceLinks struct {
	Directory DeviceDirectory
}

func (links DeviceLinks) Linked(domainId string, memberId string, deviceId string) (bool, error) {
	if deviceId == "" {
		return false, nil
	}
	err := links.valid(deviceId)
	if err != nil {
		return false, err
	}
	owners, err := links.Directory.Owners(domainId, []string{deviceId})
	if err != nil {
		return false, err
	}
	return links.owned(owners[deviceId], memberId)
}

func (links DeviceLinks) Recheck(connection Connection, now time.Time) (ConnectionRenewal, error) {
	if !links.due(connection, now) {
		return ConnectionRenewal{}, nil
	}
	linked, err := links.Linked(connection.DomainId, connection.MemberId, connection.DeviceId)
	if err != nil {
		return ConnectionRenewal{}, err
	}
	return links.renewal(connection, linked, now)
}

func (DeviceLinks) due(connection Connection, now time.Time) bool {
	return connection.DeviceId != "" && !now.Before(TimeText{}.At(connection.CheckedAt).Add(SignalLimits{}.DeviceRecheck()))
}

func (DeviceLinks) renewal(connection Connection, linked bool, now time.Time) (ConnectionRenewal, error) {
	if connection.Linked && !linked {
		return ConnectionRenewal{}, Unauthenticated{}.DeviceRemoved()
	}
	return ConnectionRenewal{Linked: linked, CheckedAt: TimeText{}.Format(now)}, nil
}

func (DeviceLinks) valid(deviceId string) error {
	if !(IdText{}).Fits(deviceId, 1) {
		return Invalid{}.DeviceIdInvalid()
	}
	return nil
}

func (DeviceLinks) owned(owner string, memberId string) (bool, error) {
	if owner != "" && owner != memberId {
		return false, Unauthenticated{}.DeviceOfAnotherPerson()
	}
	return owner == memberId, nil
}
