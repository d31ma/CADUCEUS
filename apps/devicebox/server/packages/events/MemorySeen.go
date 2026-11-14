package events

type MemorySeen struct {
	keys map[string]bool
}

func NewMemorySeen() *MemorySeen {
	return &MemorySeen{keys: map[string]bool{}}
}

func (seen *MemorySeen) Seen(domainId string, eventId string) (bool, error) {
	return seen.keys[seen.key(domainId, eventId)], nil
}

func (seen *MemorySeen) Remember(domainId string, eventId string) error {
	seen.keys[seen.key(domainId, eventId)] = true
	return nil
}

func (seen *MemorySeen) Count() int {
	return len(seen.keys)
}

func (*MemorySeen) key(domainId string, eventId string) string {
	return domainId + " " + eventId
}
