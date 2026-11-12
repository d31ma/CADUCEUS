package realtime

import "mesail.dev/packages/collections"

type ChannelBook struct {
	Storage Storage
}

func (book ChannelBook) Members(domainId string, conversationId string) ([]string, error) {
	rows, err := book.rows(domainId, map[string]any{book.conversationField(): conversationId})
	return MemberSet{}.Sorted(book.memberIds(rows)), err
}

func (book ChannelBook) Add(domainId string, conversationId string, memberId string) error {
	row, found, err := book.find(domainId, conversationId, memberId)
	if err != nil {
		return err
	}
	return book.insertUnless(found, row)
}

func (book ChannelBook) Remove(domainId string, conversationId string, memberId string) error {
	row, found, err := book.find(domainId, conversationId, memberId)
	if err != nil {
		return err
	}
	return book.deleteIf(found, row)
}

func (book ChannelBook) find(domainId string, conversationId string, memberId string) (ChannelMember, bool, error) {
	collection := book.collection(domainId)
	err := book.shelf().Open(collection)
	if err != nil {
		return ChannelMember{}, false, err
	}
	row := ChannelMember{DomainId: domainId, ConversationId: conversationId, MemberId: memberId}
	found, err := book.shelf().First(collection, map[string]any{book.conversationField(): conversationId, book.memberField(): memberId}, &row)
	return row, found, err
}

func (book ChannelBook) rows(domainId string, where map[string]any) ([]ChannelMember, error) {
	collection := book.collection(domainId)
	err := book.shelf().Open(collection)
	if err != nil {
		return nil, err
	}
	rows := []ChannelMember{}
	err = book.shelf().Every(collection, where, &rows)
	return rows, err
}

func (book ChannelBook) insertUnless(found bool, row ChannelMember) error {
	if found {
		return nil
	}
	return book.shelf().Put(book.collection(row.DomainId), "", row)
}

func (book ChannelBook) deleteIf(found bool, row ChannelMember) error {
	if !found {
		return nil
	}
	_, err := book.Storage.Documents.DeleteById(book.collection(row.DomainId), row.ID)
	return err
}

func (ChannelBook) memberIds(rows []ChannelMember) []string {
	memberIds := []string{}
	for _, row := range rows {
		memberIds = append(memberIds, row.MemberId)
	}
	return memberIds
}

func (book ChannelBook) shelf() DocumentShelf {
	return DocumentShelf{Storage: book.Storage}
}

func (ChannelBook) collection(domainId string) string {
	return collections.CollectionName{}.ForDomain("channel-members", domainId)
}

func (ChannelBook) conversationField() string {
	return "conversationId"
}

func (ChannelBook) memberField() string {
	return "memberId"
}
