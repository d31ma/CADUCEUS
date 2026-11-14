package realtime

import (
	"errors"
	"testing"
	"time"
)

func reauth(fixture signalFixture, connectionId string, token string, deviceId string) error {
	_, err := fixture.gateway.Message(GatewayMessage{ConnectionId: connectionId, DomainId: "example.com", Body: `{"type":"auth","token":"` + token + `","deviceId":"` + deviceId + `"}`})
	return err
}

func storedConnection(t *testing.T, fixture signalFixture, connectionId string) Connection {
	t.Helper()
	connection, err := fixture.gateway.Connections.Find("example.com", connectionId)
	if err != nil {
		t.Fatal(err)
	}
	return connection
}

func laterGateway(fixture signalFixture, by time.Duration) signalFixture {
	fixture.gateway.Connections.Now = fixedClock(connectionTestTime().Add(by))
	fixture.gateway.Sessions.Connections.Now = fixture.gateway.Connections.Now
	return fixture
}

func TestAReauthMustNameTheSameDevice(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	if err := reauth(fixture, "ca", "ana-later", "a2"); err != nil {
		t.Fatal(err)
	}
	if fixture.pushes.closes["ca"] != 4401 {
		t.Fatalf("got %v", fixture.pushes.closes)
	}
}

func TestAReauthWithinFiveMinutesDoesNotAskTheDirectory(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	asked := fixture.directory.asked
	if err := reauth(fixture, "ca", "ana-later", "a1"); err != nil {
		t.Fatal(err)
	}
	if fixture.directory.asked != asked || fixture.pushes.closes["ca"] != 0 {
		t.Fatalf("asked %d, closes %v", fixture.directory.asked, fixture.pushes.closes)
	}
}

func TestAReauthAfterFiveMinutesClosesARemovedDevice(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	delete(fixture.directory.owners, "a1")
	if err := reauth(laterGateway(fixture, 5*time.Minute), "ca", "ana-later", "a1"); err != nil {
		t.Fatal(err)
	}
	if fixture.pushes.closes["ca"] != 4401 {
		t.Fatalf("got %v", fixture.pushes.closes)
	}
}

func TestAReauthAfterFiveMinutesLinksADeviceAddedMeanwhile(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("cn", "ana-token", "new-device")
	fixture.directory.owners["new-device"] = "ana"
	later := laterGateway(fixture, 5*time.Minute)
	if err := reauth(later, "cn", "ana-later", "new-device"); err != nil {
		t.Fatal(err)
	}
	connection := storedConnection(t, later, "cn")
	if !connection.Linked || connection.CheckedAt != "2026-11-10T12:05:00Z" || connection.ExpiresAt != "2026-11-10T12:02:00Z" {
		t.Fatalf("got %+v", connection)
	}
}

func TestAReauthKeepsAnUnlinkedDeviceUnlinked(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("cn", "ana-token", "new-device")
	later := laterGateway(fixture, 5*time.Minute)
	if err := reauth(later, "cn", "ana-later", "new-device"); err != nil {
		t.Fatal(err)
	}
	if storedConnection(t, later, "cn").Linked || later.pushes.closes["cn"] != 0 {
		t.Fatalf("got %+v", storedConnection(t, later, "cn"))
	}
}

func TestAReauthFailsWhileTheDirectoryIsDown(t *testing.T) {
	fixture := newSignalFixture()
	fixture.device("ca", "ana-token", "a1")
	fixture.directory.down = true
	if err := reauth(laterGateway(fixture, 6*time.Minute), "ca", "ana-later", "a1"); !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}
