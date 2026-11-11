package mailin

import (
	"testing"
	"time"

	"mesail.dev/packages/outbox"
)

func TestTheFactoryWiresTheGivenPusher(t *testing.T) {
	pusher := outbox.NewRecordingPusher()
	memory := newMemory()
	inbox := DeskFactory{Environment: goodSettings(), Pusher: pusher}.Inbox(memory, memory.Page)
	inboundId, err := inbox.Accept(testEnvelope("example.com", "example.com/inbound-1"))
	check(t, err == nil && inboundId == "in_D1", "accept failed")
	check(t, len(pusher.PushedTo("messaging")) == 1, "want one push")
}

func TestTheFactoryFallsBackToTheHttpPusher(t *testing.T) {
	memory := newMemory()
	desk := DeskFactory{Environment: goodSettings()}.Outbox(memory, memory.Page)
	_, isHttp := desk.Pusher.(outbox.HTTPPusher)
	check(t, isHttp, "want the HTTP pusher")
}

func TestTheFactoryBuildsAMessageDesk(t *testing.T) {
	memory := newMemory()
	desk := DeskFactory{Environment: goodSettings()}.Messages(memory, memory.Page)
	_, missing := desk.Blobs.(MissingBlobs)
	check(t, missing && desk.LinkLife == 5*time.Minute, "want missing blobs and five minutes")
}

func TestTheFactoryWiresTheDirectoryIntoTheInbox(t *testing.T) {
	memory := newMemory()
	inbox := DeskFactory{Environment: goodSettings()}.Inbox(memory, memory.Page)
	directory, isDirectory := inbox.HandOff.Domains.(Directory)
	check(t, isDirectory && directory.Client.Timeout == DeskFactory{}.Timeout(), "want the directory client")
}
