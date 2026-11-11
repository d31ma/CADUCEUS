package channels

type Relay interface {
	Drain(collection string, id string)
}
