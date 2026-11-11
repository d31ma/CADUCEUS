package realtime

type PresenceBook struct {
	Storage Storage
}

func (book PresenceBook) Find(domainId string, memberId string) (PresenceRecord, bool, error) {
	return PresenceRecord{}, false, nil
}

func (book PresenceBook) Save(record PresenceRecord) error {
	return nil
}

func (book PresenceBook) ForMembers(domainId string, memberIds []string) ([]PresenceRecord, error) {
	return []PresenceRecord{}, nil
}

func (book PresenceBook) Due(domainId string, now string, limit int) ([]PresenceRecord, error) {
	return []PresenceRecord{}, nil
}
