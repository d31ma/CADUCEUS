package realtime

type ConnectionGateway interface {
	Push(connectionId string, event Event) error
	Send(connectionId string, frame any) error
	Close(connectionId string, code int) error
}
