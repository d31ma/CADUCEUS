package realtime

import "slices"

type SeatBook struct {
	Storage Storage
}

func (book SeatBook) Seats(domainId string, callId string) ([]SeatRecord, error) {
	found := []SeatRecord{}
	err := book.shelf().All(domainId, map[string]any{"callId": callId}, &found)
	return found, err
}

func (book SeatBook) Add(domainId string, seat SeatRecord) error {
	return book.shelf().Insert(domainId, seat)
}

func (book SeatBook) Remove(domainId string, seats []SeatRecord) error {
	for _, seat := range seats {
		err := book.shelf().Delete(domainId, seat.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (SeatBook) Of(seats []SeatRecord, deviceId string) []SeatRecord {
	return slices.DeleteFunc(slices.Clone(seats), func(seat SeatRecord) bool { return seat.DeviceId != deviceId })
}

func (SeatBook) People(seats []SeatRecord) []string {
	people := []string{}
	for _, seat := range seats {
		people = append(people, seat.PersonId)
	}
	return MemberSet{}.Sorted(people)
}

func (book SeatBook) shelf() DomainShelf {
	return DomainShelf{Storage: book.Storage, Base: "callseats"}
}
