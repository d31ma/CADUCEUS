package events

type Route struct {
	Producer       string
	OtherProducers []string
	Consumers      []string
}
