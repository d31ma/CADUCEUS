package addresses

import (
	"encoding/json"
	"testing"
)

func TestReservationJsonForPerson(t *testing.T) {
	text, err := json.Marshal(Reservation{LocalPart: "ana", Owner: "person", PersonId: "person-1"})
	assertAccepted(t, err)
	assertText(t, string(text), `{"localPart":"ana","owner":"person","personId":"person-1"}`)
}

func TestReservationJsonForAddresses(t *testing.T) {
	text, err := json.Marshal(Reservation{LocalPart: "sales", Owner: "addresses", DomainIds: []string{"a.example"}})
	assertAccepted(t, err)
	assertText(t, string(text), `{"localPart":"sales","owner":"addresses","domainIds":["a.example"]}`)
}
