package system

import (
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/platform/buildinfo"
)

func TestSnapshotTracksReadinessAndUptime(t *testing.T) {
	started := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	service := New(buildinfo.New("1", "abc", "now"), started)
	if got := service.Snapshot(started.Add(time.Second)); got.Status != "starting" || got.UptimeMS != 1000 {
		t.Fatalf("starting snapshot = %+v", got)
	}
	service.MarkReady(true)
	if got := service.Snapshot(started.Add(2 * time.Second)); got.Status != "ready" || got.APIVersion != "v1" {
		t.Fatalf("ready snapshot = %+v", got)
	}
	service.MarkReady(false)
	if service.Ready() {
		t.Fatal("service remained ready after shutdown began")
	}
}
