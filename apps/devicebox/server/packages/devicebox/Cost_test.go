package devicebox

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestOnePutForTwoHundredDevicesCostsAFewWrites(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	devices := []string{}
	for person := 0; person < 50; person++ {
		for device := 0; device < 4; device++ {
			id := fmt.Sprintf("p%02d-d%d", person, device)
			w.directory.Owns(w.domain, fmt.Sprintf("person-%02d", person), id)
			devices = append(devices, id)
		}
	}
	w.mustPut("ana", "a1", devices...)
	w.storage.Reset()
	w.mustPut("ana", "a1", devices...)
	t.Logf("a put for %d devices of 50 people: %d writes, %d queries, %d reads", len(devices), w.storage.Writes(), w.storage.Queries(), w.storage.Reads())
	if w.storage.Writes() > 11 {
		t.Fatalf("a put for 200 devices took %d writes", w.storage.Writes())
	}
	if len(w.stored("realtime")) != 100 {
		t.Fatalf("got %d events", len(w.stored("realtime")))
	}
	page := w.list("person-07", "p07-d3")
	if len(page.Envelopes) != 2 {
		t.Fatalf("got %d", len(page.Envelopes))
	}
}

func TestListingOneDeviceReadsOnlyItsOwnEnvelopesThroughTheIndex(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1")
	w.directory.Owns(w.domain, "cy", "c1")
	for index := 0; index < 40; index++ {
		w.mustPut("ana", "a1", "c1")
	}
	mine := []string{w.mustPut("ana", "a1", "b1"), w.mustPut("ana", "a1", "b1")}
	w.storage.Reset()
	page := w.list("bo", "b1")
	if len(page.Envelopes) != 2 || page.Envelopes[0].EnvelopeId != mine[0] {
		t.Fatalf("got %+v", page)
	}
	if w.storage.Queries() != 1 || w.storage.Reads() != 2 || w.storage.Writes() != 0 {
		t.Fatalf("queries %d, reads %d, writes %d", w.storage.Queries(), w.storage.Reads(), w.storage.Writes())
	}
	statement, _ := SqlText{}.WaitingFor(Names{}.Slips(w.domain), DeviceKey{}.Of("b1"), 0, 50)
	plan := w.fylo.explain(statement.Sql, statement.Parameters)
	access, _ := plan["access"].([]any)
	first, _ := access[0].(map[string]any)
	if first["kind"] != "prefix-index" || first["field"] != "waiting/"+(DeviceKey{}).Of("b1") {
		t.Fatalf("FYLO would not use the index: %v", plan)
	}
}

func TestTheLargestPutStaysUnder512KiBPerDocument(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	devices := []string{}
	for person := 0; person < 50; person++ {
		for device := 0; device < 4; device++ {
			id := fmt.Sprintf("0f8fad5b-d9cb-469f-a165-%04d%08d", person, device)
			w.directory.Owns(w.domain, fmt.Sprintf("3e5a11d0-0000-4000-8000-%012d", person), id)
			devices = append(devices, id)
		}
	}
	w.pusher.FailFor("realtime", errors.New("down"))
	w.pusher.FailFor("notify", errors.New("down"))
	id := w.envelopeId()
	_, err := w.mailbox.Put(w.member("ana"), PutRequest{EnvelopeId: id, SenderDeviceId: "a1", ToDeviceIds: devices, Kind: "message", Ciphertext: w.ciphertext(256 * 1024)})
	if err != nil {
		t.Fatal(err)
	}
	for _, collection := range []string{Names{}.Bodies(w.domain), Names{}.Slips(w.domain)} {
		rows := []map[string]any{}
		statement, _ := SqlText{}.firstWhere(collection, "envelopeId", id)
		NewDocuments(w.fylo).Rows(statement, &rows)
		encoded, _ := json.Marshal(rows[0])
		t.Logf("%s: %d bytes", collection, len(encoded))
		if len(encoded) >= 512*1024 {
			t.Fatalf("%s is %d bytes", collection, len(encoded))
		}
	}
	bodies := []Body{}
	statement, _ := SqlText{}.firstWhere(Names{}.Bodies(w.domain), "envelopeId", id)
	NewDocuments(w.fylo).Rows(statement, &bodies)
	if bodies[0].OutboxPending != 50 || len(bodies[0].PendingEvents) != 50 {
		t.Fatalf("got %d waiting events", bodies[0].OutboxPending)
	}
}
