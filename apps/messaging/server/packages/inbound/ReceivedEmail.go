package inbound

import "mesail.dev/packages/events"

type ReceivedEmail struct {
	DomainId string
	Mail     events.EmailReceived
	SendKey  string
}
