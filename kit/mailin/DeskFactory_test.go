package mailin

import (
	"net/http"
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

func TestTheFactoryFallsBackToTheSingleUsePusher(t *testing.T) {
	memory := newMemory()
	desk := DeskFactory{Environment: goodSettings()}.Outbox(memory, memory.Page)
	pusher, isEventPusher := desk.Pusher.(EventPusher)
	check(t, isEventPusher && pusher.Client.Timeout == outbox.HTTPPusher{}.DefaultTimeout(), "want the event pusher")
	check(t, keepAlivesOff(pusher.Client), "the pusher keeps connections alive")
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
	check(t, keepAlivesOff(directory.Client), "the directory client keeps connections alive")
}

func keepAlivesOff(client *http.Client) bool {
	own, isTransport := client.Transport.(*http.Transport)
	return isTransport && own.DisableKeepAlives && own != http.DefaultTransport
}
