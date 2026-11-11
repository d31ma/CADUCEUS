package core

import "fmt"

type MemoryConversations struct {
	rows    []Conversation
	counter int
}

func NewMemoryConversations() *MemoryConversations {
	return &MemoryConversations{}
}

func (store *MemoryConversations) Insert(row Conversation) (string, error) {
	store.counter++
	row.Id = fmt.Sprintf("%012d", store.counter)
	store.rows = append(store.rows, row)
	return row.Id, nil
}

func (store *MemoryConversations) All() []Conversation {
	return append([]Conversation{}, store.rows...)
}

func (store *MemoryConversations) find(match func(Conversation) bool) (Conversation, bool, error) {
	for _, row := range store.rows {
		if match(row) {
			return row, true, nil
		}
	}
	return Conversation{}, false, nil
}

func (store *MemoryConversations) filter(match func(Conversation) bool) ([]Conversation, error) {
	kept := []Conversation{}
	for _, row := range store.rows {
		kept = append(kept, store.ifMatching(row, match(row))...)
	}
	return kept, nil
}

func (store *MemoryConversations) ifMatching(row Conversation, matched bool) []Conversation {
	if matched {
		return []Conversation{row}
	}
	return nil
}

func (store *MemoryConversations) replace(row Conversation) error {
	for index := range store.rows {
		store.rows[index] = store.replaced(store.rows[index], row)
	}
	return nil
}

func (store *MemoryConversations) replaced(old Conversation, row Conversation) Conversation {
	if old.Id == row.Id {
		return row
	}
	return old
}

func (store *MemoryConversations) Save(conversation Conversation) error {
	return store.replace(conversation)
}

func (store *MemoryConversations) Find(domainId string, storedPart string) (Conversation, bool, error) {
	return store.find(func(row Conversation) bool {
		return storedPart != "" && row.DomainId == domainId && (row.Id == storedPart || row.Key == storedPart)
	})
}

func (store *MemoryConversations) ByChannel(domainId string, channelId string) (Conversation, bool, error) {
	return store.find(func(row Conversation) bool { return row.DomainId == domainId && row.ChannelId == channelId })
}

func (store *MemoryConversations) ByParticipantKey(domainId string, participantKey string) (Conversation, bool, error) {
	return store.find(func(row Conversation) bool {
		return row.DomainId == domainId && row.ParticipantKey == participantKey
	})
}
