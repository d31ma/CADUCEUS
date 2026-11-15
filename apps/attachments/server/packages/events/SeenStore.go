package events

type SeenStore interface {
	Seen(domainId string, eventId string) (bool, error)
	Remember(domainId string, eventId string) error
}
