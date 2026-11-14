package realtime

import "slices"

type Signals struct {
	Connections ConnectionBook
	Directory   DeviceDirectory
	Pairings    PairingBook
	Pushes      ConnectionGateway
}

func (signals Signals) Send(connection Connection, message ClientMessage) error {
	err := SignalCheck{}.Signal(message)
	if err != nil {
		return err
	}
	targets := MemberSet{}.Sorted(message.ToDeviceIds)
	err = signals.allowed(connection, targets)
	if err != nil {
		return err
	}
	found, err := signals.Connections.InDomain(connection.DomainId)
	if err != nil {
		return err
	}
	frame := SignalFrame{Type: FrameTypes{}.Signal(), FromDeviceId: connection.DeviceId, FromPersonId: connection.MemberId, Kind: message.Kind, Ciphertext: message.Ciphertext}
	split, err := FrameDelivery{Pushes: signals.Pushes}.Split(ConnectionGroups{}.ByDevice(found), targets, frame)
	if err != nil {
		return err
	}
	result := SignalResult{Type: FrameTypes{}.SignalResult(), RequestId: message.RequestId, Delivered: split.Reached, Offline: split.Missed}
	return signals.Pushes.Send(connection.ConnectionId, result)
}

func (signals Signals) allowed(connection Connection, targets []string) error {
	err := SignalCheck{}.Device(connection)
	if err != nil {
		return err
	}
	peers, err := signals.Pairings.Peers(connection.DomainId, connection.DeviceId)
	if err != nil {
		return err
	}
	return signals.members(connection, signals.without(targets, peers))
}

func (signals Signals) members(connection Connection, others []string) error {
	if len(others) == 0 {
		return nil
	}
	if !connection.Linked {
		return MissingTarget{}.TargetNotFound()
	}
	owners, err := signals.Directory.Owners(connection.DomainId, others)
	if err != nil {
		return err
	}
	return signals.owned(owners, others)
}

func (Signals) owned(owners map[string]string, deviceIds []string) error {
	missing := slices.ContainsFunc(deviceIds, func(deviceId string) bool { return owners[deviceId] == "" })
	if missing {
		return MissingTarget{}.TargetNotFound()
	}
	return nil
}

func (Signals) without(targets []string, peers []string) []string {
	return slices.DeleteFunc(slices.Clone(targets), func(deviceId string) bool { return slices.Contains(peers, deviceId) })
}
