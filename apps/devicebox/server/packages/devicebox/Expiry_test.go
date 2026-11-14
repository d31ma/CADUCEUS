package devicebox

import (
	"reflect"
	"testing"
	"time"
)

func TestExpiryDropsOldEnvelopesAndLeavesAnUndeliveredNoticeForTheSender(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1", "b2")
	id := w.mustPut("ana", "a1", "b1", "b2")
	w.mailbox.Acknowledge(w.member("bo"), "b1", []string{id})
	w.clock = w.clock.Add(24 * time.Hour)
	young := w.mustPut("ana", "a1", "b1")
	w.clock = w.clock.Add(29*24*time.Hour - time.Second)
	report, err := w.expiry.Run(w.service())
	if err != nil || report.Writes != 0 || report.More {
		t.Fatalf("nothing is old yet: %+v %v", report, err)
	}
	w.clock = w.clock.Add(time.Second)
	report, err = w.expiry.Run(w.service())
	if err != nil || report.Writes == 0 || report.More {
		t.Fatalf("got %+v %v", report, err)
	}
	if len(w.list("bo", "b2").Envelopes) != 0 || !reflect.DeepEqual(w.ids(w.list("bo", "b1")), []string{young}) {
		t.Fatal("the old envelope is still waiting, or the young one is gone")
	}
	notice := w.list("ana", "a1")
	if len(notice.Envelopes) != 1 || notice.Envelopes[0].EnvelopeId != id || notice.Envelopes[0].Kind != "undelivered" || notice.Envelopes[0].Ciphertext != "" {
		t.Fatalf("got %+v", notice)
	}
	stored := w.stored("realtime")
	last := stored[len(stored)-1]
	if last.ToPersonId != "ana" || last.Kind != "undelivered" || !reflect.DeepEqual(last.ToDeviceIds, []string{"a1"}) {
		t.Fatalf("got %+v", last)
	}
}

func TestAnUndeliveredNoticeExpiresWithoutANewNotice(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1")
	w.mustPut("ana", "a1", "b1")
	w.clock = w.clock.Add(31 * 24 * time.Hour)
	w.expiry.Run(w.service())
	w.clock = w.clock.Add(31 * 24 * time.Hour)
	if _, err := w.expiry.Run(w.service()); err != nil {
		t.Fatal(err)
	}
	if len(w.list("ana", "a1").Envelopes) != 0 || w.countRows(Names{}.Slips(w.domain), "kind", "undelivered") != 0 {
		t.Fatal("the notice was not dropped, or a notice made another one")
	}
}

func TestExpiryStaysWithinTheWriteBudgetAndSaysMore(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1")
	for index := 0; index < 12; index++ {
		w.mustPut("ana", "a1", "b1")
	}
	w.clock = w.clock.Add(31 * 24 * time.Hour)
	calls := 0
	for more := true; more; calls++ {
		w.storage.Reset()
		report, err := w.expiry.Run(w.service())
		if err != nil || report.Writes > (Limits{}).WriteBudget() || w.storage.Writes() > (Limits{}).WriteBudget()+1 {
			t.Fatalf("call %d: %+v %v, store writes %d", calls, report, err, w.storage.Writes())
		}
		more = report.More
	}
	if calls < 2 || len(w.list("bo", "b1").Envelopes) != 0 || len(w.list("ana", "a1").Envelopes) != 12 {
		t.Fatalf("calls %d", calls)
	}
}

func TestExpiryRemovesBodiesLeftWithoutASlip(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1")
	w.mustPut("ana", "a1", "b1")
	store := NewStore(NewDocuments(w.fylo))
	orphan, err := store.InsertBody(w.domain, Body{EnvelopeId: "orphan", DomainId: w.domain, ExpiresUs: w.clock.UnixMicro(), PendingEvents: []string{}, DeadEvents: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	w.clock = w.clock.Add(time.Hour)
	if _, err := w.expiry.Run(w.service()); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := store.Body(w.domain, orphan); found {
		t.Fatal("the orphan body is still stored")
	}
	if len(w.list("bo", "b1").Envelopes) != 1 {
		t.Fatal("a young envelope lost its body")
	}
}

func TestAnInstallWideJobCoversEveryDomainAndADomainJobOnlyItsOwn(t *testing.T) {
	w := newWorld(t)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1")
	w.directory.Owns("example.org", "ana", "a9")
	w.directory.Owns("example.org", "cy", "c9")
	w.mustPut("ana", "a1", "b1")
	w.mailbox.Put("example.org ana Member", PutRequest{EnvelopeId: w.envelopeId(), SenderDeviceId: "a9", ToDeviceIds: []string{"c9"}, Kind: "linking", Ciphertext: w.ciphertext(8)})
	w.clock = w.clock.Add(31 * 24 * time.Hour)
	w.expiry.Run("example.org job Service")
	if len(w.list("bo", "b1").Envelopes) != 1 {
		t.Fatal("a domain job touched another domain")
	}
	w.expiry.Run("* job Service")
	if len(w.list("bo", "b1").Envelopes) != 0 {
		t.Fatal("the install-wide job skipped a domain")
	}
}
