// @Service
type ConnectionStoreService struct {
	// @Needs
	dataSource DataSource
}

func (service *ConnectionStoreService) Add(connection Connection) error {
	return nil
}

func (service *ConnectionStoreService) Find(domainId string, connectionId string) (Connection, error) {
	return Connection{}, &NotFound{What: "connection"}
}

func (service *ConnectionStoreService) Renew(domainId string, connectionId string, expiresAt string) error {
	return nil
}

func (service *ConnectionStoreService) Remove(domainId string, connectionId string) (Connection, error) {
	return Connection{}, nil
}

func (service *ConnectionStoreService) CountForMember(domainId string, memberId string) (int, error) {
	return 0, nil
}

func (service *ConnectionStoreService) ForMembers(domainId string, memberIds []string) ([]Connection, error) {
	return []Connection{}, nil
}

func (service *ConnectionStoreService) InDomain(domainId string) ([]Connection, error) {
	return []Connection{}, nil
}

func (service *ConnectionStoreService) Expired(domainId string, now string, limit int) ([]Connection, error) {
	return []Connection{}, nil
}
