package realtime

import (
	"testing"
	"time"
)

func callSweepOf(fixture signalFixture) CallSweep {
	storage := fixture.memory.storage()
	return CallSweep{Calls: CallBook{Storage: storage}, Seats: SeatBook{Storage: storage}}
}

func TestTheSweepEndsACallWithNoJoinOrLeaveForTwelveHours(t *testing.T) {
	fixture, callId := callFixture(t)
	report, err := callSweepOf(fixture).Clean("example.com", connectionTestTime().Add(12*time.Hour), 25)
	if err != nil || report != (SweepReport{EndedCalls: 1, Writes: 2}) {
		t.Fatalf("got %+v, %v", report, err)
	}
	if err := fixture.send("cb", callBody("call.join", callId)); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "cb", "call.join", "not_found", "[MS5035]: ")
}

func TestTheSweepKeepsACallThatChangedLately(t *testing.T) {
	fixture, callId := callFixture(t)
	fixture.clock.move(6 * time.Hour)
	if err := fixture.send("cb", callBody("call.join", callId)); err != nil {
		t.Fatal(err)
	}
	report, err := callSweepOf(fixture).Clean("example.com", connectionTestTime().Add(12*time.Hour), 25)
	if err != nil || report != (SweepReport{}) {
		t.Fatalf("got %+v, %v", report, err)
	}
}

func TestTheSweepStopsWhenACallNeedsMoreWritesThanAreLeft(t *testing.T) {
	fixture, callId := callFixture(t)
	fixture.seatPeople(callId, 3)
	report, err := callSweepOf(fixture).Clean("example.com", connectionTestTime().Add(13*time.Hour), 4)
	if err != nil || report != (SweepReport{More: true}) {
		t.Fatalf("got %+v, %v", report, err)
	}
}

func TestTheSweepDoesNothingWithNoWritesLeft(t *testing.T) {
	fixture, _ := callFixture(t)
	report, err := callSweepOf(fixture).Clean("example.com", connectionTestTime().Add(13*time.Hour), 0)
	if err != nil || report != (SweepReport{}) {
		t.Fatalf("got %+v, %v", report, err)
	}
}

func TestTheSweepReportsStorageThatIsDown(t *testing.T) {
	fixture, _ := callFixture(t)
	fixture.memory.failAll = true
	if _, err := callSweepOf(fixture).Clean("example.com", connectionTestTime().Add(13*time.Hour), 25); err == nil {
		t.Fatal("expected an error")
	}
}
