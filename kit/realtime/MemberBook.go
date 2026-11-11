package realtime

import "mesail.dev/packages/collections"

type MemberBook struct {
	Storage Storage
}

func (book MemberBook) Members(domainId string, conversationId string) ([]string, error) {
	current, err := book.find(domainId, conversationId)
	return MemberSet{}.Sorted(current.MemberIds), err
}

func (book MemberBook) Replace(domainId string, conversationId string, memberIds []string) error {
	current, err := book.find(domainId, conversationId)
	if err != nil {
		return err
	}
	return book.store(current, MemberSet{}.Sorted(memberIds))
}

func (book MemberBook) Add(domainId string, conversationId string, memberId string) error {
	current, err := book.find(domainId, conversationId)
	if err != nil {
		return err
	}
	return book.store(current, MemberSet{}.With(current.MemberIds, memberId))
}

func (book MemberBook) Remove(domainId string, conversationId string, memberId string) error {
	current, err := book.find(domainId, conversationId)
	if err != nil {
		return err
	}
	return book.store(current, MemberSet{}.Without(current.MemberIds, memberId))
}

func (book MemberBook) find(domainId string, conversationId string) (ConversationMembers, error) {
	collection := book.collection(domainId)
	err := book.shelf().Open(collection)
	if err != nil {
		return ConversationMembers{}, err
	}
	current := ConversationMembers{DomainId: domainId, ConversationId: conversationId, MemberIds: []string{}}
	_, err = book.shelf().First(collection, map[string]any{"conversationId": conversationId}, &current)
	return current, err
}

func (book MemberBook) store(current ConversationMembers, memberIds []string) error {
	if (MemberSet{}).Same(current.MemberIds, memberIds) {
		return nil
	}
	changed := ConversationMembers{DomainId: current.DomainId, ConversationId: current.ConversationId, MemberIds: memberIds}
	return book.shelf().Put(book.collection(current.DomainId), current.ID, changed)
}

func (book MemberBook) shelf() DocumentShelf {
	return DocumentShelf{Storage: book.Storage}
}

func (MemberBook) collection(domainId string) string {
	return collections.CollectionName{}.ForDomain("conversation-members", domainId)
}
