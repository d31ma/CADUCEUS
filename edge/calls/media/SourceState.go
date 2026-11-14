package media

import (
	"slices"
	"sync"
	"time"
)

type SourceState struct {
	deviceId    string
	joinedAt    time.Time
	lock        sync.Mutex
	lastSpokeAt time.Time
	screenSince time.Time
	heard       map[FeedKey]time.Time
	ssrcs       map[FeedKey]uint32
}

func NewSourceState(deviceId string, joinedAt time.Time) *SourceState {
	return &SourceState{deviceId: deviceId, joinedAt: joinedAt, heard: map[FeedKey]time.Time{}, ssrcs: map[FeedKey]uint32{}}
}

func (state *SourceState) Track(feed FeedKey, ssrc uint32) {
	state.lock.Lock()
	defer state.lock.Unlock()
	state.ssrcs[feed] = ssrc
}

func (state *SourceState) Ssrc(feed FeedKey) (uint32, bool) {
	state.lock.Lock()
	defer state.lock.Unlock()
	ssrc, found := state.ssrcs[feed]
	return ssrc, found
}

func (state *SourceState) Heard(feed FeedKey, loud bool, now time.Time) {
	state.lock.Lock()
	defer state.lock.Unlock()
	state.screenStarted(feed, now)
	state.heard[feed] = now
	state.spoke(loud, now)
}

func (state *SourceState) Snapshot(now time.Time) SourceSnapshot {
	state.lock.Lock()
	defer state.lock.Unlock()
	return SourceSnapshot{
		DeviceId:      state.deviceId,
		JoinedAt:      state.joinedAt,
		LastSpokeAt:   state.lastSpokeAt,
		HasMicrophone: state.hasMicrophone(),
		CameraRids:    state.cameraRids(now),
		ScreenSince:   state.activeScreenSince(now),
	}
}

func (state *SourceState) screenStarted(feed FeedKey, now time.Time) {
	if feed.Kind != (FeedKinds{}).Screen() || state.active(feed, now) {
		return
	}
	state.screenSince = now
}

func (state *SourceState) spoke(loud bool, now time.Time) {
	if loud {
		state.lastSpokeAt = now
	}
}

func (state *SourceState) hasMicrophone() bool {
	_, found := state.ssrcs[FeedKey{DeviceId: state.deviceId, Kind: FeedKinds{}.Microphone()}]
	return found
}

func (state *SourceState) cameraRids(now time.Time) []string {
	rids := []string{}
	for feed, at := range state.heard {
		rids = append(rids, state.liveCameraRid(feed, at, now)...)
	}
	slices.Sort(rids)
	return rids
}

func (state *SourceState) liveCameraRid(feed FeedKey, at time.Time, now time.Time) []string {
	if feed.Kind != (FeedKinds{}).Camera() || now.Sub(at) >= state.activeWindow() {
		return nil
	}
	return []string{feed.Rid}
}

func (state *SourceState) activeScreenSince(now time.Time) time.Time {
	if !state.active(FeedKey{DeviceId: state.deviceId, Kind: FeedKinds{}.Screen()}, now) {
		return time.Time{}
	}
	return state.screenSince
}

func (state *SourceState) active(feed FeedKey, now time.Time) bool {
	return now.Sub(state.heard[feed]) < state.activeWindow()
}

func (*SourceState) activeWindow() time.Duration {
	return 2 * time.Second
}
