package channels

import (
	"errors"
	"testing"
)

func quietWorldWithChannels(t *testing.T) *world {
	t.Helper()
	w := newQuietWorld(t)
	w.general = w.create("Bearer ana", "general", "public")
	w.must(w.join("Bearer cy", w.general))
	w.must(w.leave("Bearer cy", w.general))
	return w
}

func TestEveryStorageFailureDuringCreateSurfaces(t *testing.T) {
	everyStorageFailureSurfaces(t, newQuietWorld, func(w *world) error {
		_, err := w.workflow.Create("Bearer ana", CreateRequest{Name: "general", Visibility: "public"})
		return err
	})
}

func TestEveryStorageFailureDuringListSurfaces(t *testing.T) {
	everyStorageFailureSurfaces(t, quietWorldWithChannels, func(w *world) error {
		_, err := w.workflow.List("Bearer bo")
		return err
	})
}

func TestEveryStorageFailureDuringJoinSurfaces(t *testing.T) {
	everyStorageFailureSurfaces(t, quietWorldWithChannels, func(w *world) error {
		return w.join("Bearer bo", w.general)
	})
}

func TestEveryStorageFailureDuringRejoinSurfaces(t *testing.T) {
	everyStorageFailureSurfaces(t, quietWorldWithChannels, func(w *world) error {
		return w.join("Bearer cy", w.general)
	})
}

func TestEveryStorageFailureDuringLeaveSurfaces(t *testing.T) {
	everyStorageFailureSurfaces(t, quietWorldWithChannels, func(w *world) error {
		return w.leave("Bearer ana", w.general)
	})
}

func TestEveryStorageFailureDuringInviteSurfaces(t *testing.T) {
	everyStorageFailureSurfaces(t, quietWorldWithChannels, func(w *world) error {
		return w.invite("Bearer ana", w.general, "bo", "cy")
	})
}

func TestEveryStorageFailureDuringUpdateSurfaces(t *testing.T) {
	everyStorageFailureSurfaces(t, quietWorldWithChannels, func(w *world) error {
		_, err := w.update("Bearer dee", w.general, UpdateRequest{Archived: flag(true)})
		return err
	})
}

func worldWithOutboxWork(t *testing.T) *world {
	t.Helper()
	w := newWorld(t)
	w.pusher.FailFor("search", permanentFailure())
	w.pusher.FailFor("messaging", errors.New("messaging is down"))
	w.general = w.create("Bearer ana", "general", "public")
	w.pusher.Recover("search")
	w.deadEventId = w.firstDeadEventId()
	return w
}

func TestEveryStorageFailureDuringTheSweepSurfaces(t *testing.T) {
	everyStorageFailureSurfaces(t, worldWithOutboxWork, func(w *world) error {
		_, err := w.desk.RunOnce()
		return err
	})
}

func TestEveryStorageFailureWhileCountingWaitingEventsSurfaces(t *testing.T) {
	everyStorageFailureSurfaces(t, worldWithOutboxWork, func(w *world) error {
		_, err := w.desk.Waiting()
		return err
	})
}

func TestEveryStorageFailureWhileListingDeadEventsSurfaces(t *testing.T) {
	everyStorageFailureSurfaces(t, worldWithOutboxWork, func(w *world) error {
		_, err := w.desk.DeadEvents()
		return err
	})
}

func TestEveryStorageFailureDuringARetrySurfaces(t *testing.T) {
	everyStorageFailureSurfaces(t, worldWithOutboxWork, func(w *world) error {
		return w.desk.RetryDead(Names{}.Channels("example.com"), w.general.Id, w.deadEventId, "search")
	})
}

func (w *world) firstDeadEventId() string {
	w.t.Helper()
	dead, err := w.desk.DeadEvents()
	w.must(err)
	return dead[0].EventId
}
