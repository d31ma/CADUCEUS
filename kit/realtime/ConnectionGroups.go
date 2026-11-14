package realtime

type ConnectionGroups struct{}

func (ConnectionGroups) ByDevice(found []Connection) map[string][]Connection {
	groups := map[string][]Connection{}
	for _, connection := range found {
		groups[connection.DeviceId] = append(groups[connection.DeviceId], connection)
	}
	return groups
}

func (ConnectionGroups) ByMember(found []Connection) map[string][]Connection {
	groups := map[string][]Connection{}
	for _, connection := range found {
		groups[connection.MemberId] = append(groups[connection.MemberId], connection)
	}
	return groups
}
