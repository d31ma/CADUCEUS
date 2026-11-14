package media

import (
	"sync"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

type SlotOutput struct {
	track      *webrtc.TrackLocalStaticRTP
	wait       time.Duration
	lock       sync.Mutex
	current    FeedKey
	pending    FeedKey
	pendingAt  time.Time
	continuity *Continuity
}

func NewSlotOutput(track *webrtc.TrackLocalStaticRTP, kind SlotKind) *SlotOutput {
	return &SlotOutput{track: track, wait: kind.Wait, continuity: NewContinuity(kind.Step)}
}

func (slot *SlotOutput) Current() FeedKey {
	slot.lock.Lock()
	defer slot.lock.Unlock()
	return slot.current
}

func (slot *SlotOutput) Shown() string {
	slot.lock.Lock()
	defer slot.lock.Unlock()
	return slot.wanted().DeviceId
}

func (slot *SlotOutput) Feeds() []FeedKey {
	slot.lock.Lock()
	defer slot.lock.Unlock()
	return []FeedKey{slot.current, slot.pending}
}

func (slot *SlotOutput) Target(feed FeedKey, now time.Time) bool {
	slot.lock.Lock()
	defer slot.lock.Unlock()
	return slot.retarget(feed, now)
}

func (slot *SlotOutput) Forward(feed FeedKey, packet *rtp.Packet, frame FrameStart, now time.Time) error {
	slot.lock.Lock()
	defer slot.lock.Unlock()
	slot.promote(feed, frame, now)
	return slot.write(feed, packet)
}

func (slot *SlotOutput) wanted() FeedKey {
	if slot.pending.Empty() {
		return slot.current
	}
	return slot.pending
}

func (slot *SlotOutput) retarget(feed FeedKey, now time.Time) bool {
	switch {
	case feed == slot.current:
		slot.pending = FeedKey{}
		return false
	case feed.Empty():
		slot.current, slot.pending = FeedKey{}, FeedKey{}
		return false
	case feed == slot.pending:
		return false
	case slot.current.Empty():
		slot.current = feed
		slot.continuity.Restart()
		return true
	}
	slot.pending, slot.pendingAt = feed, now
	return true
}

func (slot *SlotOutput) promote(feed FeedKey, frame FrameStart, now time.Time) {
	if !slot.ready(feed, frame, now) {
		return
	}
	slot.current, slot.pending = slot.pending, FeedKey{}
	slot.continuity.Restart()
}

func (slot *SlotOutput) ready(feed FeedKey, frame FrameStart, now time.Time) bool {
	return !feed.Empty() && feed == slot.pending && (frame.Independent || frame.New && now.Sub(slot.pendingAt) >= slot.wait)
}

func (slot *SlotOutput) write(feed FeedKey, packet *rtp.Packet) error {
	if feed != slot.current {
		return nil
	}
	copied := slot.copied(packet)
	slot.continuity.Rewrite(&copied.Header)
	return slot.track.WriteRTP(copied)
}

func (*SlotOutput) copied(packet *rtp.Packet) *rtp.Packet {
	copied := *packet
	copied.Header.Extension = false
	copied.Header.ExtensionProfile = 0
	copied.Header.Extensions = nil
	return &copied
}
