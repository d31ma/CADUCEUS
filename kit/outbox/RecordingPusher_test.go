package outbox

import (
	"errors"
	"testing"
)

func TestRecordingPusherRecordsPushesAndFailures(t *testing.T) {
	pusher := NewRecordingPusher()
	pusher.FailFor("realtime", errors.New("down"))

	first := pusher.Push(Subscriber{Name: "messaging"}, channelCreated("evt_1"))
	failed := pusher.Push(Subscriber{Name: "realtime"}, channelCreated("evt_1"))
	pusher.Recover("realtime")
	recovered := pusher.Push(Subscriber{Name: "realtime"}, channelCreated("evt_2"))

	if first != nil || failed == nil || recovered != nil {
		t.Fatalf("errors %v %v %v", first, failed, recovered)
	}
	if len(pusher.Attempted()) != 3 || len(pusher.Pushed()) != 2 {
		t.Fatalf("attempted %+v, pushed %+v", pusher.Attempted(), pusher.Pushed())
	}
	toRealtime := pusher.PushedTo("realtime")
	if len(toRealtime) != 1 || toRealtime[0].EventID != "evt_2" {
		t.Fatalf("pushed to realtime %+v", toRealtime)
	}
}
