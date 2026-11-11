// @Service
type PresenceStoreService struct {
	// @Needs
	dataSource DataSource
}

func (service *PresenceStoreService) Find(domainId string, memberId string) (PresenceRecord, bool, error) {
	return PresenceRecord{}, false, nil
}

func (service *PresenceStoreService) Save(record PresenceRecord) error {
	return nil
}

func (service *PresenceStoreService) ForMembers(domainId string, memberIds []string) ([]PresenceRecord, error) {
	return []PresenceRecord{}, nil
}

func (service *PresenceStoreService) Due(domainId string, now string, limit int) ([]PresenceRecord, error) {
	return []PresenceRecord{}, nil
}
