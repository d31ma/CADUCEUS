package realtime

type ConnectionGateway interface {
	Push(connectionId string, event Event) error
	Close(connectionId string, code int) error
}
