package outbox

import (
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"mesail.dev/packages/events"
)

type slowPusher struct {
	mutex    sync.Mutex
	waits    map[string][]time.Duration
	answers  map[string][]error
	started  []Push
	answered []Push
}

func newSlowPusher() *slowPusher {
	return &slowPusher{waits: map[string][]time.Duration{}, answers: map[string][]error{}}
}

func (pusher *slowPusher) waitFirst(subscriber string, wait time.Duration) {
	pusher.waits[subscriber] = append(pusher.waits[subscriber], wait)
}

func (pusher *slowPusher) answerFirst(subscriber string, err error) {
	pusher.answers[subscriber] = append(pusher.answers[subscriber], err)
}

func (pusher *slowPusher) Push(subscriber Subscriber, envelope events.Envelope) error {
	wait, answer := pusher.begin(subscriber.Name, envelope)
	time.Sleep(wait)
	pusher.mutex.Lock()
	defer pusher.mutex.Unlock()
	pusher.answered = append(pusher.answered, Push{Subscriber: subscriber.Name, Envelope: envelope})
	return answer
}

func (pusher *slowPusher) begin(name string, envelope events.Envelope) (time.Duration, error) {
	pusher.mutex.Lock()
	defer pusher.mutex.Unlock()
	pusher.started = append(pusher.started, Push{Subscriber: name, Envelope: envelope})
	wait, answer := time.Duration(0), error(nil)
	if len(pusher.waits[name]) > 0 {
		wait, pusher.waits[name] = pusher.waits[name][0], pusher.waits[name][1:]
	}
	if len(pusher.answers[name]) > 0 {
		answer, pusher.answers[name] = pusher.answers[name][0], pusher.answers[name][1:]
	}
	return wait, answer
}

func (pusher *slowPusher) startedTo(name string) []string {
	pusher.mutex.Lock()
	defer pusher.mutex.Unlock()
	ids := []string{}
	for _, push := range pusher.started {
		if push.Subscriber == name {
			ids = append(ids, push.Envelope.EventID)
		}
	}
	return ids
}

type countingStore struct {
	*MemoryStore
	saves int
}

func (store *countingStore) Save(ref Ref, fields Fields) error {
	store.saves++
	return store.MemoryStore.Save(ref, fields)
}

type drainFixture struct {
	store  *countingStore
	pusher *slowPusher
	now    time.Time
	relay  Relay
	ref    Ref
}

func newDrainFixture(t *testing.T, envelopes ...events.Envelope) *drainFixture {
	t.Helper()
	fixture := &drainFixture{store: &countingStore{MemoryStore: NewMemoryStore()}, pusher: newSlowPusher(), now: startTime(), ref: Ref{Collection: "channels", Id: "chn_1"}}
	fixture.store.Put(fixture.ref, stageAll(t, testSettings(), envelopes...))
	fixture.relay = NewRelay(fixture.store, fixture.pusher, testSettings(), func() time.Time { return fixture.now })
	return fixture
}

func (fixture *drainFixture) drain(t *testing.T) (Report, time.Duration) {
	t.Helper()
	started := time.Now()
	report, err := fixture.relay.RelayDocument(fixture.ref)
	if err != nil {
		t.Fatal(err)
	}
	return report, time.Since(started)
}

func (fixture *drainFixture) waiting(t *testing.T) map[string][]string {
	t.Helper()
	state, err := Codec{}.Read(fixture.store.Fields(fixture.ref))
	if err != nil {
		t.Fatal(err)
	}
	waiting := map[string][]string{}
	for _, event := range state.Pending {
		waiting[event.Envelope.EventID] = event.Waiting
	}
	return waiting
}

func (fixture *drainFixture) attempts(t *testing.T, eventId string) int {
	t.Helper()
	state, _ := Codec{}.Read(fixture.store.Fields(fixture.ref))
	for _, event := range state.Pending {
		if event.Envelope.EventID == eventId {
			return event.Attempts
		}
	}
	return -1
}

func TestDrainReturnsWithinItsBudgetAndTheRelayDeliversTheSlowPushLater(t *testing.T) {
	fixture := newDrainFixture(t, channelCreated("evt_1"))
	fixture.pusher.waitFirst("realtime", 10*time.Second)

	report, took := fixture.drain(t)

	if took > 3500*time.Millisecond {
		t.Fatalf("drain on write took %v", took)
	}
	if report.Delivered != 1 || fixture.store.saves != 1 {
		t.Fatalf("report %+v, saves %d", report, fixture.store.saves)
	}
	if waiting := fixture.waiting(t); !slices.Equal(waiting["evt_1"], []string{"realtime"}) || fixture.attempts(t, "evt_1") != 0 {
		t.Fatalf("waiting %+v, attempts %d", waiting, fixture.attempts(t, "evt_1"))
	}
	relayed, err := fixture.relay.RunOnce(10)
	if err != nil || relayed.Delivered != 1 || len(fixture.waiting(t)) != 0 {
		t.Fatalf("relay %+v %v, waiting %+v", relayed, err, fixture.waiting(t))
	}
}

func TestDrainPushesToDifferentSubscribersAtTheSameTime(t *testing.T) {
	fixture := newDrainFixture(t, channelCreated("evt_1"))
	fixture.pusher.waitFirst("messaging", 1500*time.Millisecond)
	fixture.pusher.waitFirst("realtime", 1500*time.Millisecond)

	report, took := fixture.drain(t)

	if took > 2500*time.Millisecond || report.Delivered != 2 {
		t.Fatalf("took %v, report %+v", took, report)
	}
}

func TestDrainPushesToOneSubscriberInOrderAndStopsAtAFailure(t *testing.T) {
	fixture := newDrainFixture(t, channelCreated("evt_1"), channelCreated("evt_2"), channelCreated("evt_3"))
	fixture.pusher.answerFirst("realtime", nil)
	fixture.pusher.answerFirst("realtime", DeliveryFailure{Subscriber: "realtime", Status: http.StatusServiceUnavailable})

	report, _ := fixture.drain(t)

	if got := fixture.pusher.startedTo("realtime"); !slices.Equal(got, []string{"evt_1", "evt_2"}) {
		t.Fatalf("realtime got %v", got)
	}
	if got := fixture.pusher.startedTo("messaging"); !slices.Equal(got, []string{"evt_1", "evt_2", "evt_3"}) {
		t.Fatalf("messaging got %v", got)
	}
	waiting := fixture.waiting(t)
	if report.Delivered != 4 || report.Retried != 1 || len(waiting) != 2 || fixture.attempts(t, "evt_2") != 1 || fixture.attempts(t, "evt_3") != 0 {
		t.Fatalf("report %+v, waiting %+v", report, waiting)
	}
}

func TestDrainLeavesATooManyRequestsAnswerPendingForTheRelay(t *testing.T) {
	fixture := newDrainFixture(t, channelCreated("evt_1"))
	fixture.pusher.answerFirst("messaging", DeliveryFailure{Subscriber: "messaging", Status: http.StatusTooManyRequests})

	report, _ := fixture.drain(t)

	if report.Retried != 1 || report.DeadLettered != 0 || !slices.Equal(fixture.waiting(t)["evt_1"], []string{"messaging"}) {
		t.Fatalf("report %+v, waiting %+v", report, fixture.waiting(t))
	}
}

func TestDrainGivesEachPushTwoSecondsAndStartsNoLaterPushForThatSubscriber(t *testing.T) {
	fixture := newDrainFixture(t, channelCreated("evt_1"), channelCreated("evt_2"))
	fixture.pusher.waitFirst("realtime", 2500*time.Millisecond)

	report, took := fixture.drain(t)

	if took > 2500*time.Millisecond || report.Delivered != 2 {
		t.Fatalf("took %v, report %+v", took, report)
	}
	if got := fixture.pusher.startedTo("realtime"); !slices.Equal(got, []string{"evt_1"}) {
		t.Fatalf("realtime got %v", got)
	}
	waiting := fixture.waiting(t)
	if !slices.Equal(waiting["evt_1"], []string{"realtime"}) || !slices.Equal(waiting["evt_2"], []string{"realtime"}) {
		t.Fatalf("waiting %+v", waiting)
	}
}

func TestDrainStopsTheWholeDrainAfterThreeSeconds(t *testing.T) {
	fixture := newDrainFixture(t, channelCreated("evt_1"), channelCreated("evt_2"), channelCreated("evt_3"))
	fixture.pusher.waitFirst("realtime", 1200*time.Millisecond)
	fixture.pusher.waitFirst("realtime", 1200*time.Millisecond)
	fixture.pusher.waitFirst("realtime", 1200*time.Millisecond)

	report, took := fixture.drain(t)

	if took > 3500*time.Millisecond || report.Delivered != 5 {
		t.Fatalf("took %v, report %+v", took, report)
	}
	if waiting := fixture.waiting(t); !slices.Equal(waiting["evt_3"], []string{"realtime"}) || len(waiting) != 1 {
		t.Fatalf("waiting %+v", waiting)
	}
}

func TestDrainSkipsAnEventThatIsNotDueYet(t *testing.T) {
	fixture := newDrainFixture(t, channelCreated("evt_1"))
	fixture.pusher.answerFirst("realtime", DeliveryFailure{Subscriber: "realtime", Status: http.StatusServiceUnavailable})
	fixture.drain(t)

	fixture.drain(t)

	if got := fixture.pusher.startedTo("realtime"); len(got) != 1 {
		t.Fatalf("realtime got %v", got)
	}
}

func TestSweepDocumentWaitsForASlowSubscriber(t *testing.T) {
	fixture := newDrainFixture(t, channelCreated("evt_1"))
	fixture.pusher.waitFirst("realtime", 2500*time.Millisecond)

	report, err := fixture.relay.SweepDocument(fixture.ref)

	if err != nil || report.Delivered != 2 || len(fixture.waiting(t)) != 0 {
		t.Fatalf("report %+v %v, waiting %+v", report, err, fixture.waiting(t))
	}
}

func TestPushFuncCallsTheFunction(t *testing.T) {
	called := ""
	push := PushFunc(func(subscriber Subscriber, envelope events.Envelope) error {
		called = subscriber.Name + "/" + envelope.EventID
		return nil
	})

	err := push.Push(Subscriber{Name: "search"}, channelCreated("evt_9"))

	if err != nil || called != "search/evt_9" {
		t.Fatalf("called %q, err %v", called, err)
	}
}
