package realtime

import "mesail.dev/packages/collections"

type DirectBook struct {
	Storage Storage
}

func (book DirectBook) Members(domainId string, conversationId string) ([]string, error) {
	current, err := book.find(domainId, conversationId)
	return MemberSet{}.Sorted(current.MemberIds), err
}

func (book DirectBook) Replace(domainId string, conversationId string, memberIds []string) error {
	current, err := book.find(domainId, conversationId)
	if err != nil {
		return err
	}
	return book.store(current, MemberSet{}.Sorted(memberIds))
}

func (book DirectBook) find(domainId string, conversationId string) (ConversationMembers, error) {
	collection := book.collection(domainId)
	err := book.shelf().Open(collection)
	if err != nil {
		return ConversationMembers{}, err
	}
	current := ConversationMembers{DomainId: domainId, ConversationId: conversationId, MemberIds: []string{}}
	_, err = book.shelf().First(collection, map[string]any{"conversationId": conversationId}, &current)
	return current, err
}

func (book DirectBook) store(current ConversationMembers, memberIds []string) error {
	if (MemberSet{}).Same(current.MemberIds, memberIds) {
		return nil
	}
	changed := ConversationMembers{DomainId: current.DomainId, ConversationId: current.ConversationId, MemberIds: memberIds}
	return book.shelf().Put(book.collection(current.DomainId), current.ID, changed)
}

func (book DirectBook) shelf() DocumentShelf {
	return DocumentShelf{Storage: book.Storage}
}

func (DirectBook) collection(domainId string) string {
	return collections.CollectionName{}.ForDomain("conversation-members", domainId)
}
