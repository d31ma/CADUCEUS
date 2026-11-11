package realtime

import "time"

type ConnectionBook struct {
	Storage Storage
	Now     func() time.Time
}

func (book ConnectionBook) Add(connection Connection) error {
	return nil
}

func (book ConnectionBook) Find(domainId string, connectionId string) (Connection, error) {
	return Connection{}, Missing{What: "connection"}
}

func (book ConnectionBook) Renew(domainId string, connectionId string, expiresAt string) error {
	return nil
}

func (book ConnectionBook) Remove(domainId string, connectionId string) (Connection, error) {
	return Connection{}, nil
}

func (book ConnectionBook) CountForMember(domainId string, memberId string) (int, error) {
	return 0, nil
}

func (book ConnectionBook) ForMembers(domainId string, memberIds []string) ([]Connection, error) {
	return []Connection{}, nil
}

func (book ConnectionBook) InDomain(domainId string) ([]Connection, error) {
	return []Connection{}, nil
}

func (book ConnectionBook) Expired(domainId string, now string, limit int) ([]Connection, error) {
	return []Connection{}, nil
}
