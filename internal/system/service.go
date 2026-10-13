// Package system owns process-local runtime state exposed by the system API.
package system

import (
	"sync/atomic"
	"time"

	"github.com/d31ma/caduceus/internal/platform/buildinfo"
)

// Snapshot is the versioned, transport-neutral system view.
type Snapshot struct {
	APIVersion string         `json:"apiVersion"`
	Status     string         `json:"status"`
	StartedAt  time.Time      `json:"startedAt"`
	UptimeMS   int64          `json:"uptimeMs"`
	Build      buildinfo.Info `json:"build"`
}

// Service tracks readiness independently from HTTP transport lifecycle.
type Service struct {
	info      buildinfo.Info
	startedAt time.Time
	ready     atomic.Bool
}

// New constructs a process state service.
func New(info buildinfo.Info, startedAt time.Time) *Service {
	return &Service{info: info, startedAt: startedAt.UTC()}
}

// MarkReady changes the externally observable readiness state.
func (service *Service) MarkReady(ready bool) {
	service.ready.Store(ready)
}

// Ready reports whether required dependencies and listeners are ready.
func (service *Service) Ready() bool {
	return service.ready.Load()
}

// Snapshot returns a stable v1 system document.
func (service *Service) Snapshot(now time.Time) Snapshot {
	status := "starting"
	if service.Ready() {
		status = "ready"
	}
	uptime := now.Sub(service.startedAt)
	if uptime < 0 {
		uptime = 0
	}
	return Snapshot{
		APIVersion: "v1",
		Status:     status,
		StartedAt:  service.startedAt,
		UptimeMS:   uptime.Milliseconds(),
		Build:      service.info,
	}
}
