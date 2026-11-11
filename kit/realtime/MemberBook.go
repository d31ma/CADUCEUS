package realtime

type MemberBook struct {
	Storage Storage
}

func (book MemberBook) Members(domainId string, conversationId string) ([]string, error) {
	return []string{}, nil
}

func (book MemberBook) Replace(domainId string, conversationId string, memberIds []string) error {
	return nil
}

func (book MemberBook) Add(domainId string, conversationId string, memberId string) error {
	return nil
}

func (book MemberBook) Remove(domainId string, conversationId string, memberId string) error {
	return nil
}
