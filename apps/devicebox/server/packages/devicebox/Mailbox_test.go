package devicebox

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestAPutWaitsInEachReceivingBoxAndIsListedWithItsCiphertext(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1", "b2")
	w.directory.Owns(w.domain, "cy", "c1")
	receipt, id, err := w.put("ana", "a1", "b1", "b2", "c1")
	if err != nil || receipt.EnvelopeId != id || receipt.ExpiresAt != "2026-12-14T09:00:00Z" {
		t.Fatalf("got %+v %v", receipt, err)
	}
	for _, box := range [][2]string{{"bo", "b1"}, {"bo", "b2"}, {"cy", "c1"}} {
		page := w.list(box[0], box[1])
		if !reflect.DeepEqual(w.ids(page), []string{id}) || page.Envelopes[0].Ciphertext == "" || page.Envelopes[0].SenderPersonId != "ana" || page.Next != "" {
			t.Fatalf("%v got %+v", box, page)
		}
	}
	if len(w.list("ana", "a1").Envelopes) != 0 {
		t.Fatal("the sender's box should be empty")
	}
}

func TestAPutStagesOneEnvelopeStoredPerPersonWithoutCiphertextOrSender(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1", "b2")
	w.directory.Owns(w.domain, "cy", "c1")
	_, id, _ := w.put("ana", "a1", "b1", "b2", "c1")
	stored := w.stored("realtime")
	if len(stored) != 2 || len(w.stored("notify")) != 2 {
		t.Fatalf("got %+v", stored)
	}
	if stored[0].ToPersonId != "bo" || !reflect.DeepEqual(stored[0].ToDeviceIds, []string{"b1", "b2"}) || stored[0].EnvelopeId != id || stored[0].Kind != "message" {
		t.Fatalf("got %+v", stored[0])
	}
	if stored[1].ToPersonId != "cy" || !reflect.DeepEqual(stored[1].ToDeviceIds, []string{"c1"}) {
		t.Fatalf("got %+v", stored[1])
	}
	encoded, _ := json.Marshal(stored)
	if strings.Contains(string(encoded), `"a1"`) || strings.Contains(string(encoded), "ana") || strings.Contains(string(encoded), "ciphertext") {
		t.Fatalf("an event names the sender or holds ciphertext: %s", encoded)
	}
}

func TestTheSamePutTwiceIsAnsweredLikeTheFirstAndStoredOnce(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1")
	request := PutRequest{EnvelopeId: w.envelopeId(), SenderDeviceId: "a1", ToDeviceIds: []string{"b1"}, Kind: "group", Ciphertext: w.ciphertext(10)}
	first, err := w.mailbox.Put(w.member("ana"), request)
	request.EnvelopeId = strings.ToUpper(request.EnvelopeId)
	second, secondError := w.mailbox.Put(w.member("ana"), request)
	if err != nil || secondError != nil || first != second {
		t.Fatalf("got %+v %+v %v %v", first, second, err, secondError)
	}
	if len(w.list("bo", "b1").Envelopes) != 1 {
		t.Fatal("stored twice")
	}
}

func TestTheSameIdWithOtherContentIsADuplicate(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1")
	request := PutRequest{EnvelopeId: w.envelopeId(), SenderDeviceId: "a1", ToDeviceIds: []string{"b1"}, Kind: "message", Ciphertext: w.ciphertext(10)}
	w.mailbox.Put(w.member("ana"), request)
	request.Ciphertext = w.ciphertext(10)
	_, err := w.mailbox.Put(w.member("ana"), request)
	if numberOf(err) != 6012 {
		t.Fatalf("got %v", err)
	}
}

func TestOnlyDevicesOfThisDomainMayTakePart(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1")
	w.directory.Owns("other.org", "eve", "e1")
	cases := []struct {
		person string
		sender string
		to     string
		number int
	}{
		{"bo", "a1", "b1", 6010},
		{"ana", "e1", "b1", 6010},
		{"ana", "a1", "e1", 6011},
		{"ana", "a1", "unknown", 6011},
	}
	for _, testCase := range cases {
		_, _, err := w.put(testCase.person, testCase.sender, testCase.to)
		if numberOf(err) != testCase.number {
			t.Fatalf("%+v got %v", testCase, err)
		}
	}
}

func TestOnlyTheDevicesPersonMayListOrAcknowledgeItsBox(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1")
	id := w.mustPut("ana", "a1", "b1")
	_, listError := w.mailbox.List(w.member("ana"), "b1", "", "")
	acknowledgeError := w.mailbox.Acknowledge(w.member("ana"), "b1", []string{id})
	_, strangerError := w.mailbox.List("other.org bo Member", "b1", "", "")
	_, badIdError := w.mailbox.List(w.member("bo"), "b 1", "", "")
	for _, err := range []error{listError, acknowledgeError, strangerError, badIdError} {
		if numberOf(err) != 6014 {
			t.Fatalf("got %v", err)
		}
	}
	if len(w.list("bo", "b1").Envelopes) != 1 {
		t.Fatal("a refused acknowledgement removed the envelope")
	}
}

func TestCallersMustBeSignedInMembers(t *testing.T) {
	w := newWorld(t)
	_, err := w.mailbox.Put("garbage", PutRequest{})
	_, serviceError := w.mailbox.List(w.service(), "b1", "", "")
	acknowledgeError := w.mailbox.Acknowledge("garbage", "b1", nil)
	_, expireError := w.expiry.Run("garbage")
	if numberOf(err) != 6001 || numberOf(serviceError) != 6002 || numberOf(acknowledgeError) != 6001 || numberOf(expireError) != 6001 {
		t.Fatalf("got %v %v %v %v", err, serviceError, acknowledgeError, expireError)
	}
}

func TestPagesComeOldestFirstAndFollowTheCursor(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1")
	first := w.mustPut("ana", "a1", "b1")
	second := w.mustPut("ana", "a1", "b1")
	third := w.mustPut("ana", "a1", "b1")
	page, err := w.mailbox.List(w.member("bo"), "b1", "", "2")
	if err != nil || !reflect.DeepEqual(w.ids(page), []string{first, second}) || page.Next == "" {
		t.Fatalf("got %+v %v", page, err)
	}
	rest, err := w.mailbox.List(w.member("bo"), "b1", page.Next, "2")
	if err != nil || !reflect.DeepEqual(w.ids(rest), []string{third}) || rest.Next != "" {
		t.Fatalf("got %+v %v", rest, err)
	}
}

func TestAPageStopsAtItsCiphertextBudget(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1")
	for index := 0; index < 13; index++ {
		w.mailbox.Put(w.member("ana"), PutRequest{EnvelopeId: w.envelopeId(), SenderDeviceId: "a1", ToDeviceIds: []string{"b1"}, Kind: "message", Ciphertext: w.ciphertext(256 * 1024)})
	}
	page := w.list("bo", "b1")
	if len(page.Envelopes) != 12 || page.Next == "" {
		t.Fatalf("got %d envelopes, next %q", len(page.Envelopes), page.Next)
	}
	rest, _ := w.mailbox.List(w.member("bo"), "b1", page.Next, "")
	if len(rest.Envelopes) != 1 || rest.Next != "" {
		t.Fatalf("got %d, next %q", len(rest.Envelopes), rest.Next)
	}
}

func TestAnAcknowledgementRemovesOnlyThatDeviceAndTheLastOneDeletesTheEnvelope(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1", "b2")
	id := w.mustPut("ana", "a1", "b1", "b2")
	if err := w.mailbox.Acknowledge(w.member("bo"), "b1", []string{id}); err != nil {
		t.Fatal(err)
	}
	if len(w.list("bo", "b1").Envelopes) != 0 || len(w.list("bo", "b2").Envelopes) != 1 {
		t.Fatal("the acknowledgement did not remove exactly one device")
	}
	if w.countRows(Names{}.Slips(w.domain), "envelopeId", id) != 1 || w.countRows(Names{}.Bodies(w.domain), "envelopeId", id) != 1 {
		t.Fatal("the envelope was deleted while a device still waits")
	}
	if err := w.mailbox.Acknowledge(w.member("bo"), "b2", []string{strings.ToUpper(id), id}); err != nil {
		t.Fatal(err)
	}
	if w.countRows(Names{}.Slips(w.domain), "envelopeId", id) != 0 || w.countRows(Names{}.Bodies(w.domain), "envelopeId", id) != 0 {
		t.Fatal("the envelope is still stored after the last device")
	}
}

func TestAcknowledgingIdsThatDoNotWaitChangesNothing(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1")
	if err := w.mailbox.Acknowledge(w.member("bo"), "b1", []string{w.envelopeId()}); err != nil {
		t.Fatalf("an empty domain answered %v", err)
	}
	w.directory.Owns(w.domain, "cy", "c1")
	id := w.mustPut("ana", "a1", "c1")
	w.storage.Reset()
	if err := w.mailbox.Acknowledge(w.member("bo"), "b1", []string{id, w.envelopeId()}); err != nil {
		t.Fatal(err)
	}
	if w.storage.Writes() != 0 || len(w.list("cy", "c1").Envelopes) != 1 {
		t.Fatalf("writes %d", w.storage.Writes())
	}
}

func TestTheBoxCountFollowsPutsAndAcknowledgements(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1")
	counts := NewBoxCounts(NewDocuments(w.fylo))
	id := w.mustPut("ana", "a1", "b1")
	w.mustPut("ana", "a1", "b1")
	key := DeviceKey{}.Of("b1")
	read := func() int {
		buckets, _ := counts.read(w.domain, counts.bucketsOf([]string{key}))
		return buckets[DeviceKey{}.Bucket(key)].Counts[key]
	}
	if read() != 2 {
		t.Fatalf("got %d", read())
	}
	w.mailbox.Acknowledge(w.member("bo"), "b1", []string{id})
	if read() != 1 {
		t.Fatalf("got %d", read())
	}
}

func TestAFullBoxRefusesMoreEnvelopes(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1")
	w.directory.Owns(w.domain, "cy", "c1")
	w.mustPut("ana", "a1", "b1")
	counts := NewBoxCounts(NewDocuments(w.fylo))
	if _, err := counts.Add(w.domain, map[string]int{DeviceKey{}.Of("b1"): 9998}); err != nil {
		t.Fatal(err)
	}
	w.mustPut("ana", "a1", "b1")
	_, _, err := w.put("ana", "a1", "c1", "b1")
	if numberOf(err) != 6013 {
		t.Fatalf("got %v", err)
	}
	if len(w.list("cy", "c1").Envelopes) != 0 {
		t.Fatal("a refused put was stored for another device")
	}
}

func TestADirectoryFailureIsUnavailable(t *testing.T) {
	w := newWorld(t)
	w.directory.Fail(true)
	_, _, err := w.put("ana", "a1", "b1")
	_, listError := w.mailbox.List(w.member("ana"), "a1", "", "")
	if numberOf(err) != 6019 || numberOf(listError) != 6019 {
		t.Fatalf("got %v %v", err, listError)
	}
	w.mailbox.ownership = NewOwnership(UnwiredDirectory{})
	_, _, err = w.put("ana", "a1", "b1")
	if numberOf(err) != 6020 {
		t.Fatalf("got %v", err)
	}
}
