package realtime

type DomainBook struct {
	Storage Storage
}

func (book DomainBook) Remember(domainId string) error {
	return nil
}

func (book DomainBook) List(cursor string, limit int) ([]string, string, error) {
	return []string{}, "", nil
}
