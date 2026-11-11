package outbox

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestCodecSetsTheSweepFlagsWithTheCounts(t *testing.T) {
	waiting, _ := Codec{}.Write(OutboxState{Pending: []PendingEvent{{Envelope: channelCreated("evt_1"), Waiting: []string{"messaging"}}}})
	dead, _ := Codec{}.Write(OutboxState{Dead: []DeadLetter{{Envelope: channelCreated("evt_2"), Subscriber: "realtime"}}})
	empty, _ := Codec{}.Write(OutboxState{})

	if !waiting.OutboxWaiting || waiting.OutboxHasDead {
		t.Fatalf("waiting fields %+v", waiting)
	}
	if dead.OutboxWaiting || !dead.OutboxHasDead {
		t.Fatalf("dead fields %+v", dead)
	}
	if empty.OutboxWaiting || empty.OutboxHasDead {
		t.Fatalf("empty fields %+v", empty)
	}
}

func TestFiltersMatchTheSweepFlagsByExactValue(t *testing.T) {
	waiting, _ := json.Marshal(Filters{}.Waiting())
	dead, _ := json.Marshal(Filters{}.HasDead())

	if string(waiting) != `{"outboxWaiting":true}` || string(dead) != `{"outboxHasDead":true}` {
		t.Fatalf("filters %s and %s", waiting, dead)
	}
}

func TestRunOnceSaysMoreWhenTheLimitIsReached(t *testing.T) {
	fixture := newRelayFixture(t, testSettings(), channelCreated("evt_1"))
	fixture.store.Put(Ref{Collection: "channels", Id: "chn_2"}, stageAll(t, testSettings(), channelCreated("evt_2")))

	first, err := fixture.relay.RunOnce(1)
	second, _ := fixture.relay.RunOnce(5)

	if err != nil || !first.More || first.Documents != 1 || second.More || second.Documents != 1 {
		t.Fatalf("reports %+v and %+v, error %v", first, second, err)
	}
}

func TestRunWithinStopsWhenTheTimeIsSpent(t *testing.T) {
	fixture := newRelayFixture(t, testSettings(), channelCreated("evt_1"))
	fixture.store.Put(Ref{Collection: "channels", Id: "chn_2"}, stageAll(t, testSettings(), channelCreated("evt_2")))
	fixture.relay = NewRelay(fixture.store, hookPusher{inner: fixture.pusher, hook: func() { fixture.now = fixture.now.Add(time.Minute) }}, testSettings(), func() time.Time { return fixture.now })

	report, err := fixture.relay.RunWithin(10, 30*time.Second)

	if err != nil || !report.More || report.Documents != 1 {
		t.Fatalf("report %+v, error %v", report, err)
	}
	if fixture.store.Fields(Ref{Collection: "channels", Id: "chn_2"}).OutboxPending != 1 {
		t.Fatal("the late document should stay pending")
	}
}

func TestRunWithinRelaysEverythingInTime(t *testing.T) {
	fixture := newRelayFixture(t, testSettings(), channelCreated("evt_1"))

	report, err := fixture.relay.RunWithin(10, time.Minute)

	if err != nil || report.More || report.Documents != 1 || report.Delivered != 2 {
		t.Fatalf("report %+v, error %v", report, err)
	}
}

func TestRunWithinReportsAFailingPendingQuery(t *testing.T) {
	fixture := newRelayFixture(t, testSettings(), channelCreated("evt_1"))
	fixture.store.FailNextPending(errors.New("the store stopped"))

	_, err := fixture.relay.RunWithin(10, time.Minute)

	if err == nil {
		t.Fatal("a failing pending query should be reported")
	}
}

func TestDomainHTTPPusherAsksATokenForTheEnvelopeDomain(t *testing.T) {
	seen := recordedRequest{}
	server := answeringServer(t, http.StatusAccepted, &seen)
	asked := []string{}
	tokens := DomainTokenFunc(func(domainId string) (string, error) {
		asked = append(asked, domainId)
		return "token-" + domainId, nil
	})
	envelope := channelCreated("evt_1")
	envelope.DomainId = "example.org"

	err := NewDomainHTTPPusher(tokens, time.Second).Push(Subscriber{Name: "messaging", URL: server.URL}, envelope)

	if err != nil || len(asked) != 1 || asked[0] != "example.org" || seen.authorization != "Bearer token-example.org" {
		t.Fatalf("asked %v, authorization %q, error %v", asked, seen.authorization, err)
	}
}
