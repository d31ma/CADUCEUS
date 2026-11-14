package realtime

import (
	"testing"
	"time"
)

func pairingBody(frameType string, pairingId string) string {
	return `{"type":"` + frameType + `","requestId":"p1","pairingId":"` + pairingId + `"}`
}

func pairedFixture(t *testing.T) signalFixture {
	t.Helper()
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	fixture.device("cn", "ana-token", "new-device")
	if err := fixture.send("ca", pairingBody("pairing.open", "pairing-1234567890")); err != nil {
		t.Fatal(err)
	}
	if err := fixture.send("cn", pairingBody("pairing.join", "pairing-1234567890")); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestOpeningAPairingAnswersOpened(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	if err := fixture.send("ca", pairingBody("pairing.open", "pairing-1234567890")); err != nil {
		t.Fatal(err)
	}
	frame := fixture.last("ca")
	if frame["type"] != "pairing.opened" || frame["pairingId"] != "pairing-1234567890" || frame["requestId"] != "p1" {
		t.Fatalf("got %+v", frame)
	}
}

func TestTheSamePersonsNewDeviceJoinsAndBothGetReady(t *testing.T) {
	fixture := pairedFixture(t)
	opener := fixture.last("ca")
	joiner := fixture.last("cn")
	if opener["type"] != "pairing.ready" || opener["peerDeviceId"] != "new-device" || joiner["type"] != "pairing.ready" || joiner["peerDeviceId"] != "a1" || joiner["requestId"] != "p1" {
		t.Fatalf("opener %+v, joiner %+v", opener, joiner)
	}
}

func TestPairedDevicesSignalEachOtherBeforeTheNewOneIsInTheDirectory(t *testing.T) {
	fixture := pairedFixture(t)
	if err := fixture.send("cn", signalBody(`"a1"`, "linking", "aGVsbG8")); err != nil {
		t.Fatal(err)
	}
	if fixture.last("ca")["fromDeviceId"] != "new-device" || fixture.last("ca")["kind"] != "linking" {
		t.Fatalf("got %+v", fixture.last("ca"))
	}
	if err := fixture.send("ca", signalBody(`"new-device"`, "linking", "aGk")); err != nil {
		t.Fatal(err)
	}
	if fixture.last("cn")["fromDeviceId"] != "a1" {
		t.Fatalf("got %+v", fixture.last("cn"))
	}
}

func TestTheNewDeviceStillMayNotSignalOthers(t *testing.T) {
	fixture := pairedFixture(t)
	if err := fixture.send("cn", signalBody(`"a1","b1"`, "rtc", "aGVsbG8")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "cn", "signal", "not_found", "[MS5031]: ")
}

func TestAPairingEndsAfterTwoMinutes(t *testing.T) {
	fixture := pairedFixture(t)
	fixture.clock.move(2 * time.Minute)
	if err := fixture.send("cn", signalBody(`"a1"`, "linking", "aGVsbG8")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "cn", "signal", "not_found", "[MS5031]: ")
}

func TestAnotherPersonCannotJoinAPairing(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	fixture.device("cb", "ben-token", "b1")
	if err := fixture.send("ca", pairingBody("pairing.open", "pairing-1234567890")); err != nil {
		t.Fatal(err)
	}
	if err := fixture.send("cb", pairingBody("pairing.join", "pairing-1234567890")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "cb", "pairing.join", "not_found", "[MS5033]: ")
}

func TestAnExpiredPairingCannotBeJoined(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	fixture.device("cn", "ana-token", "new-device")
	if err := fixture.send("ca", pairingBody("pairing.open", "pairing-1234567890")); err != nil {
		t.Fatal(err)
	}
	fixture.clock.move(3 * time.Minute)
	if err := fixture.send("cn", pairingBody("pairing.join", "pairing-1234567890")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "cn", "pairing.join", "not_found", "[MS5033]: ")
}

func TestAnExpiredPairingIdCanBeOpenedAgain(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	fixture.device("cb", "ben-token", "b1")
	if err := fixture.send("ca", pairingBody("pairing.open", "pairing-1234567890")); err != nil {
		t.Fatal(err)
	}
	fixture.clock.move(3 * time.Minute)
	if err := fixture.send("cb", pairingBody("pairing.open", "pairing-1234567890")); err != nil {
		t.Fatal(err)
	}
	if fixture.last("cb")["type"] != "pairing.opened" {
		t.Fatalf("got %+v", fixture.last("cb"))
	}
}

func TestAHeldPairingIdCannotBeTakenByAnotherPerson(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	fixture.device("cb", "ben-token", "b1")
	if err := fixture.send("ca", pairingBody("pairing.open", "pairing-1234567890")); err != nil {
		t.Fatal(err)
	}
	if err := fixture.send("cb", pairingBody("pairing.open", "pairing-1234567890")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "cb", "pairing.open", "not_found", "[MS5033]: ")
}

func TestAThirdDeviceCannotJoinATakenPairing(t *testing.T) {
	fixture := pairedFixture(t)
	fixture.device("c3", "ana-token", "third-device")
	if err := fixture.send("c3", pairingBody("pairing.join", "pairing-1234567890")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "c3", "pairing.join", "not_found", "[MS5033]: ")
}

func TestTheOpenerCannotJoinItsOwnPairing(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	if err := fixture.send("ca", pairingBody("pairing.open", "pairing-1234567890")); err != nil {
		t.Fatal(err)
	}
	if err := fixture.send("ca", pairingBody("pairing.join", "pairing-1234567890")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "pairing.join", "not_found", "[MS5033]: ")
}

func TestJoiningAnUnknownPairingIsNotFound(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("cn", "ana-token", "new-device")
	if err := fixture.send("cn", pairingBody("pairing.join", "pairing-1234567890")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "cn", "pairing.join", "not_found", "[MS5033]: ")
}

func TestAShortPairingIdIsRefused(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	if err := fixture.send("ca", pairingBody("pairing.open", "short")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "pairing.open", "invalid_request", "[MS5032]: ")
}

func TestPairingNeedsADevice(t *testing.T) {
	fixture := newSignalFixture()
	fixture.authenticate("ca", "ana-token")
	if err := fixture.send("ca", pairingBody("pairing.open", "pairing-1234567890")); err != nil {
		t.Fatal(err)
	}
	refusedWith(t, fixture, "ca", "pairing.open", "invalid_request", "[MS5025]: ")
}

func TestPairingWhileStorageIsDownIsUnavailable(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	fixture.memory.failAll = true
	if err := fixture.send("ca", pairingBody("pairing.open", "pairing-1234567890")); err == nil {
		t.Fatal("expected the connection lookup to fail")
	}
}
