package notifications

import (
	"testing"
	"time"
)

func pushNow() time.Time {
	return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
}

func TestPushSendsWithoutPresence(t *testing.T) {
	expectSame(t, PushRules{}.Decide("", "", pushNow()), "send")
}

func TestPushSendsWhenIdle(t *testing.T) {
	expectSame(t, PushRules{}.Decide("idle", "", pushNow()), "send")
}

func TestPushSendsWhenOffline(t *testing.T) {
	expectSame(t, PushRules{}.Decide("offline", "", pushNow()), "send")
}

func TestPushSendsForUnknownState(t *testing.T) {
	expectSame(t, PushRules{}.Decide("away", "", pushNow()), "send")
}

func TestPushSkipsWhenActive(t *testing.T) {
	expectSame(t, PushRules{}.Decide("active", "", pushNow()), "skip_active")
}

func TestPushSkipsDoNotDisturbWithoutEnd(t *testing.T) {
	expectSame(t, PushRules{}.Decide("dnd", "", pushNow()), "skip_dnd")
}

func TestPushSkipsDoNotDisturbWithUnreadableEnd(t *testing.T) {
	expectSame(t, PushRules{}.Decide("dnd", "tomorrow", pushNow()), "skip_dnd")
}

func TestPushSkipsDoNotDisturbWithFutureEnd(t *testing.T) {
	expectSame(t, PushRules{}.Decide("dnd", "2026-11-10T12:00:01Z", pushNow()), "skip_dnd")
}

func TestPushSendsWhenDoNotDisturbEndsNow(t *testing.T) {
	expectSame(t, PushRules{}.Decide("dnd", "2026-11-10T12:00:00Z", pushNow()), "send")
}

func TestPushSendsWhenDoNotDisturbEnded(t *testing.T) {
	expectSame(t, PushRules{}.Decide("dnd", "2026-11-10T13:00:00+02:00", pushNow()), "send")
}
