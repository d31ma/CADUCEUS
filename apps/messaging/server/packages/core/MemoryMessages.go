package core

import "fmt"

type MemoryMessages struct {
	rows    []Message
	counter int
}

func NewMemoryMessages() *MemoryMessages {
	return &MemoryMessages{}
}

func (store *MemoryMessages) Insert(row Message) (string, error) {
	store.counter++
	row.Id = fmt.Sprintf("%012d", store.counter)
	store.rows = append(store.rows, row)
	return row.Id, nil
}

func (store *MemoryMessages) All() []Message {
	return append([]Message{}, store.rows...)
}

func (store *MemoryMessages) find(match func(Message) bool) (Message, bool, error) {
	for _, row := range store.rows {
		if match(row) {
			return row, true, nil
		}
	}
	return Message{}, false, nil
}

func (store *MemoryMessages) filter(match func(Message) bool) ([]Message, error) {
	kept := []Message{}
	for _, row := range store.rows {
		kept = append(kept, store.ifMatching(row, match(row))...)
	}
	return kept, nil
}

func (store *MemoryMessages) ifMatching(row Message, matched bool) []Message {
	if matched {
		return []Message{row}
	}
	return nil
}

func (store *MemoryMessages) replace(row Message) error {
	for index := range store.rows {
		store.rows[index] = store.replaced(store.rows[index], row)
	}
	return nil
}

func (store *MemoryMessages) replaced(old Message, row Message) Message {
	if old.Id == row.Id {
		return row
	}
	return old
}

func (store *MemoryMessages) Save(message Message) error {
	return store.replace(message)
}

func (store *MemoryMessages) Find(domainId string, month string, id string) (Message, bool, error) {
	return store.find(func(row Message) bool { return row.DomainId == domainId && row.Month == month && row.Id == id })
}

func (store *MemoryMessages) BySendKey(domainId string, month string, sendKey string) (Message, bool, error) {
	return store.find(func(row Message) bool {
		return row.DomainId == domainId && row.Month == month && row.SendKey == sendKey
	})
}

func (store *MemoryMessages) OfConversation(domainId string, month string, conversationId string) ([]Message, error) {
	return store.filter(func(row Message) bool {
		return row.DomainId == domainId && row.Month == month && row.ConversationId == conversationId
	})
}
