package media

import (
	"sync"
	"time"
)

type KeyframeGate struct {
	lock  sync.Mutex
	asked map[FeedKey]time.Time
}

func NewKeyframeGate() *KeyframeGate {
	return &KeyframeGate{asked: map[FeedKey]time.Time{}}
}

func (gate *KeyframeGate) Allow(feed FeedKey, now time.Time) bool {
	gate.lock.Lock()
	defer gate.lock.Unlock()
	return gate.allowed(feed, now)
}

func (gate *KeyframeGate) allowed(feed FeedKey, now time.Time) bool {
	if now.Sub(gate.asked[feed]) < gate.interval() {
		return false
	}
	gate.asked[feed] = now
	return true
}

func (*KeyframeGate) interval() time.Duration {
	return 250 * time.Millisecond
}
