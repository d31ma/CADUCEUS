package realtime

import (
	"errors"
	"strconv"
	"testing"
)

func fanOutFixture() (*memoryStorage, *recordingGateway, FanOut) {
	memory := newMemoryStorage()
	pushes := newRecordingGateway()
	book := connectionTestBook(memory)
	book.Add(testConnection("example.com", "ana", "a1"))
	book.Add(testConnection("example.com", "ana", "a2"))
	book.Add(testConnection("example.com", "ben", "b1"))
	book.Add(testConnection("example.com", "cid", "c1"))
	book.Add(testConnection("example.org", "ana", "other"))
	return memory, pushes, FanOut{Connections: book, Gateway: pushes}
}

func TestToMembersReachesEveryConnectionOfThoseMembersOnly(t *testing.T) {
	_, pushes, fanOut := fanOutFixture()
	err := fanOut.ToMembers("example.com", []string{"ana", "ben"}, Event{Type: "message.created"})
	if err != nil || len(pushes.eventsOf("a1")) != 1 || len(pushes.eventsOf("a2")) != 1 || len(pushes.eventsOf("b1")) != 1 || len(pushes.eventsOf("c1")) != 0 || len(pushes.eventsOf("other")) != 0 {
		t.Fatalf("got %v %+v", err, pushes.pushes)
	}
}

func TestToMembersExceptSkipsThatMember(t *testing.T) {
	_, pushes, fanOut := fanOutFixture()
	err := fanOut.ToMembersExcept("example.com", []string{"ana", "ben"}, "ana", Event{Type: "typing"})
	if err != nil || len(pushes.eventsOf("a1")) != 0 || len(pushes.eventsOf("b1")) != 1 || len(pushes.eventsOf("other")) != 0 {
		t.Fatalf("got %v %+v", err, pushes.pushes)
	}
}

func TestToDomainNeverReachesAnotherDomain(t *testing.T) {
	_, pushes, fanOut := fanOutFixture()
	err := fanOut.ToDomain("example.com", Event{Type: "presence.changed"})
	if err != nil || len(pushes.pushes) != 4 || len(pushes.eventsOf("other")) != 0 {
		t.Fatalf("got %v %+v", err, pushes.pushes)
	}
}

func TestFanOutRemovesAGoneConnectionAndStillReachesTheOthers(t *testing.T) {
	_, pushes, fanOut := fanOutFixture()
	pushes.gone["a1"] = true
	err := fanOut.ToMembers("example.com", []string{"ana", "ben"}, Event{Type: "message.created"})
	_, missing := fanOut.Connections.Find("example.com", "a1")
	if err != nil || !errors.As(missing, &Missing{}) || len(pushes.eventsOf("a2")) != 1 || len(pushes.eventsOf("b1")) != 1 {
		t.Fatalf("got %v %v %+v", err, missing, pushes.pushes)
	}
}

func TestFanOutRemovesAtMost25GoneConnections(t *testing.T) {
	memory := newMemoryStorage()
	pushes := newRecordingGateway()
	book := connectionTestBook(memory)
	for index := range 30 {
		book.Add(testConnection("example.com", "ana", "g"+strconv.Itoa(index)))
		pushes.gone["g"+strconv.Itoa(index)] = true
	}
	err := FanOut{Connections: book, Gateway: pushes}.ToDomain("example.com", Event{Type: "presence.changed"})
	left, _ := book.InDomain("example.com")
	if err != nil || len(left) != 5 {
		t.Fatalf("got %v, %d left", err, len(left))
	}
}

func TestFanOutReportsAGatewayThatIsDown(t *testing.T) {
	_, pushes, fanOut := fanOutFixture()
	pushes.down = true
	if err := fanOut.ToDomain("example.com", Event{Type: "presence.changed"}); !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}

func TestFanOutReportsStorageFailures(t *testing.T) {
	memory, pushes, fanOut := fanOutFixture()
	memory.failAll = true
	if err := fanOut.ToMembers("example.com", []string{"ana"}, Event{Type: "typing"}); !errors.As(err, &Unavailable{}) {
		t.Fatalf("to members: got %v", err)
	}
	if err := fanOut.ToMembersExcept("example.com", []string{"ana"}, "ben", Event{Type: "typing"}); !errors.As(err, &Unavailable{}) {
		t.Fatalf("to members except: got %v", err)
	}
	if err := fanOut.ToDomain("example.com", Event{Type: "typing"}); !errors.As(err, &Unavailable{}) {
		t.Fatalf("to domain: got %v", err)
	}
	if len(pushes.pushes) != 0 {
		t.Fatalf("pushed %+v", pushes.pushes)
	}
}

func TestToConnectionPushesToThatConnection(t *testing.T) {
	_, pushes, fanOut := fanOutFixture()
	err := fanOut.ToConnection("b1", Event{Type: "auth.accepted"})
	if err != nil || len(pushes.eventsOf("b1")) != 1 || len(pushes.pushes) != 1 {
		t.Fatalf("got %v %+v", err, pushes.pushes)
	}
}

func deviceConnection(domainId string, memberId string, connectionId string, deviceId string, linked bool) Connection {
	connection := testConnection(domainId, memberId, connectionId)
	connection.DeviceId = deviceId
	connection.Linked = linked
	return connection
}

func waitingFixture() (*recordingGateway, FanOut) {
	memory := newMemoryStorage()
	pushes := newRecordingGateway()
	book := connectionTestBook(memory)
	book.Add(deviceConnection("example.com", "ana", "a-listed", "dev_a1", true))
	book.Add(deviceConnection("example.com", "ana", "a-listed-again", "dev_a1", true))
	book.Add(deviceConnection("example.com", "ana", "a-other-device", "dev_a2", true))
	book.Add(deviceConnection("example.com", "ana", "a-not-linked", "dev_a3", false))
	book.Add(deviceConnection("example.com", "ana", "a-no-device", "", false))
	book.Add(deviceConnection("example.com", "ben", "b-listed-id", "dev_a1", true))
	book.Add(deviceConnection("example.org", "ana", "a-other-domain", "dev_a1", true))
	return pushes, FanOut{Connections: book, Gateway: pushes}
}

func TestToDevicesReachesTheLinkedSocketsOfTheListedDevices(t *testing.T) {
	pushes, fanOut := waitingFixture()
	err := fanOut.ToDevices("example.com", "ana", []string{"dev_a1", "dev_a3"}, Event{Type: "devicebox.waiting", State: "message"})
	if err != nil || len(pushes.eventsOf("a-listed")) != 1 || len(pushes.eventsOf("a-listed-again")) != 1 {
		t.Fatalf("got %v %+v", err, pushes.pushes)
	}
}

func TestToDevicesSkipsOtherDevicesUnlinkedSocketsAndSocketsWithoutADevice(t *testing.T) {
	pushes, fanOut := waitingFixture()
	err := fanOut.ToDevices("example.com", "ana", []string{"dev_a1", "dev_a3"}, Event{Type: "devicebox.waiting", State: "message"})
	if err != nil || len(pushes.eventsOf("a-other-device")) != 0 || len(pushes.eventsOf("a-not-linked")) != 0 || len(pushes.eventsOf("a-no-device")) != 0 {
		t.Fatalf("got %v %+v", err, pushes.pushes)
	}
}

func TestToDevicesSkipsAnotherPersonAndAnotherDomainWithTheSameDeviceId(t *testing.T) {
	pushes, fanOut := waitingFixture()
	err := fanOut.ToDevices("example.com", "ana", []string{"dev_a1"}, Event{Type: "devicebox.waiting", State: "message"})
	if err != nil || len(pushes.eventsOf("b-listed-id")) != 0 || len(pushes.eventsOf("a-other-domain")) != 0 {
		t.Fatalf("got %v %+v", err, pushes.pushes)
	}
}

func TestToDevicesWithNoListedDeviceReachesNobody(t *testing.T) {
	pushes, fanOut := waitingFixture()
	err := fanOut.ToDevices("example.com", "ana", []string{}, Event{Type: "devicebox.waiting", State: "message"})
	if err != nil || len(pushes.pushes) != 0 {
		t.Fatalf("got %v %+v", err, pushes.pushes)
	}
}
