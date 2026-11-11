package realtime

import (
	"errors"
	"slices"
)

type FanOut struct {
	Connections ConnectionBook
	Gateway     ConnectionGateway
}

func (fanOut FanOut) ToMembers(domainId string, memberIds []string, event Event) error {
	found, err := fanOut.Connections.ForMembers(domainId, memberIds)
	if err != nil {
		return err
	}
	return fanOut.deliver(domainId, found, event)
}

func (fanOut FanOut) ToMembersExcept(domainId string, memberIds []string, exceptMemberId string, event Event) error {
	found, err := fanOut.Connections.ForMembers(domainId, memberIds)
	if err != nil {
		return err
	}
	return fanOut.deliver(domainId, fanOut.without(found, exceptMemberId), event)
}

func (fanOut FanOut) ToDomain(domainId string, event Event) error {
	found, err := fanOut.Connections.InDomain(domainId)
	if err != nil {
		return err
	}
	return fanOut.deliver(domainId, found, event)
}

func (fanOut FanOut) ToConnection(connectionId string, event Event) error {
	return fanOut.Gateway.Push(connectionId, event)
}

func (fanOut FanOut) deliver(domainId string, found []Connection, event Event) error {
	removals := 0
	for _, connection := range fanOut.complete(found) {
		removed, err := fanOut.settle(domainId, connection, fanOut.Gateway.Push(connection.ConnectionId, event), removals)
		if err != nil {
			return err
		}
		removals += removed
	}
	return nil
}

func (fanOut FanOut) settle(domainId string, connection Connection, pushed error, removals int) (int, error) {
	switch {
	case errors.As(pushed, &Gone{}) && removals < fanOut.maximumRemovals():
		_, err := fanOut.Connections.Remove(domainId, connection.ConnectionId)
		return 1, err
	case errors.As(pushed, &Gone{}):
		return 0, nil
	}
	return 0, pushed
}

func (FanOut) complete(found []Connection) []Connection {
	return slices.DeleteFunc(found, func(connection Connection) bool {
		return connection.ConnectionId == ""
	})
}

func (FanOut) without(found []Connection, exceptMemberId string) []Connection {
	return slices.DeleteFunc(found, func(connection Connection) bool {
		return connection.MemberId == exceptMemberId
	})
}

func (FanOut) maximumRemovals() int {
	return 25
}
