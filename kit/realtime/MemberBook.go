package realtime

type MemberBook struct {
	Storage Storage
}

func (book MemberBook) Members(domainId string, conversationId string) ([]string, error) {
	channel, err := book.channels().Members(domainId, conversationId)
	if err != nil {
		return nil, err
	}
	return book.chosen(channel, domainId, conversationId)
}

func (book MemberBook) Channel(domainId string, conversationId string) ([]string, error) {
	return book.channels().Members(domainId, conversationId)
}

func (book MemberBook) Replace(domainId string, conversationId string, memberIds []string) error {
	return book.directs().Replace(domainId, conversationId, memberIds)
}

func (book MemberBook) Add(domainId string, conversationId string, memberId string) error {
	return book.channels().Add(domainId, conversationId, memberId)
}

func (book MemberBook) Remove(domainId string, conversationId string, memberId string) error {
	return book.channels().Remove(domainId, conversationId, memberId)
}

func (book MemberBook) chosen(channel []string, domainId string, conversationId string) ([]string, error) {
	if len(channel) > 0 {
		return channel, nil
	}
	return book.directs().Members(domainId, conversationId)
}

func (book MemberBook) channels() ChannelBook {
	return ChannelBook{Storage: book.Storage}
}

func (book MemberBook) directs() DirectBook {
	return DirectBook{Storage: book.Storage}
}
