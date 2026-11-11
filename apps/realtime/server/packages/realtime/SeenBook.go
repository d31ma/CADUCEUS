package realtime

type SeenBook struct {
	Storage Storage
}

func (book SeenBook) Seen(domainId string, eventId string) (bool, error) {
	return false, nil
}

func (book SeenBook) Remember(domainId string, eventId string) error {
	return nil
}
