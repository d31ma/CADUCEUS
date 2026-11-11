package core

import "fmt"

type MemoryParticipants struct {
	rows    []Participant
	counter int
}

func NewMemoryParticipants() *MemoryParticipants {
	return &MemoryParticipants{}
}

func (store *MemoryParticipants) Insert(row Participant) (string, error) {
	store.counter++
	row.Id = fmt.Sprintf("%012d", store.counter)
	store.rows = append(store.rows, row)
	return row.Id, nil
}

func (store *MemoryParticipants) All() []Participant {
	return append([]Participant{}, store.rows...)
}

func (store *MemoryParticipants) find(match func(Participant) bool) (Participant, bool, error) {
	for _, row := range store.rows {
		if match(row) {
			return row, true, nil
		}
	}
	return Participant{}, false, nil
}

func (store *MemoryParticipants) filter(match func(Participant) bool) ([]Participant, error) {
	kept := []Participant{}
	for _, row := range store.rows {
		kept = append(kept, store.ifMatching(row, match(row))...)
	}
	return kept, nil
}

func (store *MemoryParticipants) ifMatching(row Participant, matched bool) []Participant {
	if matched {
		return []Participant{row}
	}
	return nil
}

func (store *MemoryParticipants) replace(row Participant) error {
	for index := range store.rows {
		store.rows[index] = store.replaced(store.rows[index], row)
	}
	return nil
}

func (store *MemoryParticipants) replaced(old Participant, row Participant) Participant {
	if old.Id == row.Id {
		return row
	}
	return old
}

func (store *MemoryParticipants) Save(participant Participant) error {
	return store.replace(participant)
}

func (store *MemoryParticipants) Delete(domainId string, id string) error {
	kept, _ := store.filter(func(row Participant) bool { return row.Id != id || row.DomainId != domainId })
	store.rows = kept
	return nil
}

func (store *MemoryParticipants) OfConversation(domainId string, conversationId string) ([]Participant, error) {
	return store.filter(func(row Participant) bool {
		return row.DomainId == domainId && row.ConversationId == conversationId
	})
}

func (store *MemoryParticipants) OfMember(domainId string, memberId string) ([]Participant, error) {
	return store.filter(func(row Participant) bool { return row.DomainId == domainId && row.MemberId == memberId })
}
