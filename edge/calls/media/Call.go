package media

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtp"

	"mesail.dev/edge/calls/failures"
)

type Call struct {
	Id        string
	DomainId  string
	context   CallContext
	onIdle    func(*Call)
	lock      sync.Mutex
	members   map[string]*Member
	joining   int
	routes    atomic.Pointer[RouteTable]
	keyframes *KeyframeGate
	wake      chan struct{}
	done      chan struct{}
	stopped   sync.Once
}

func NewCall(seat Seat, context CallContext, onIdle func(*Call)) *Call {
	call := &Call{
		Id:        seat.CallId,
		DomainId:  seat.DomainId,
		context:   context,
		onIdle:    onIdle,
		members:   map[string]*Member{},
		keyframes: NewKeyframeGate(),
		wake:      make(chan struct{}, 1),
		done:      make(chan struct{}),
	}
	call.routes.Store(RouteTable{}.From(nil))
	return call
}

func (call *Call) Start() {
	go call.loop()
}

func (call *Call) Stop() {
	call.stopped.Do(call.closeDone)
}

func (call *Call) Reserve(domainId string) error {
	call.lock.Lock()
	defer call.lock.Unlock()
	return call.reserved(domainId)
}

func (call *Call) Abandon() {
	call.lock.Lock()
	defer call.lock.Unlock()
	call.joining--
}

func (call *Call) Add(member *Member) error {
	replaced, err := call.placed(member)
	if err != nil {
		return err
	}
	call.closeReplaced(replaced)
	call.Wake()
	return nil
}

func (call *Call) Member(deviceId string, domainId string) (*Member, error) {
	call.lock.Lock()
	defer call.lock.Unlock()
	return call.seated(deviceId, domainId)
}

func (call *Call) Idle() bool {
	call.lock.Lock()
	defer call.lock.Unlock()
	return call.idle()
}

func (call *Call) Size() int {
	call.lock.Lock()
	defer call.lock.Unlock()
	return len(call.members)
}

func (call *Call) Leave(member *Member) {
	call.remove(member, "left")
}

func (call *Call) Gone(member *Member) {
	call.remove(member, "connection lost")
}

func (call *Call) Deliver(feed FeedKey, packet *rtp.Packet, frame FrameStart, now time.Time) {
	call.forward(call.routes.Load().Outputs(feed), feed, packet, frame)
}

func (call *Call) RequestKeyframe(feed FeedKey) {
	call.ask(feed, call.sources(feed.DeviceId), call.keyframes.Allow(feed, call.context.Clock.Now()))
}

func (call *Call) Plan() {
	now := call.context.Clock.Now()
	members := call.memberList()
	requests := call.applyAll(members, call.snapshots(members, now), now)
	call.routes.Store(RouteTable{}.From(members))
	call.askAll(requests)
}

func (call *Call) Wake() {
	call.nudge()
}

func (call *Call) nudge() {
	select {
	case call.wake <- struct{}{}:
	default:
	}
}

func (call *Call) CloseAll() {
	call.closeMembers(call.memberList())
}

func (call *Call) closeDone() {
	close(call.done)
}

func (call *Call) loop() {
	ticker := time.NewTicker(call.context.Options.PlanInterval)
	defer ticker.Stop()
	for call.waitTurn(ticker) {
		call.Plan()
	}
}

func (call *Call) waitTurn(ticker *time.Ticker) bool {
	select {
	case <-ticker.C:
		return true
	case <-call.wake:
		return true
	case <-call.done:
		return false
	}
}

func (call *Call) idle() bool {
	return len(call.members) == 0 && call.joining == 0
}

func (call *Call) reserved(domainId string) error {
	if domainId != call.DomainId {
		return failures.Problems{}.TokenForAnotherDomain()
	}
	call.joining++
	return nil
}

func (call *Call) placed(member *Member) ([]*Member, error) {
	call.lock.Lock()
	defer call.lock.Unlock()
	call.joining--
	err := call.room(member.Seat.DeviceId)
	if err != nil {
		return nil, err
	}
	replaced := call.existing(member.Seat.DeviceId)
	call.members[member.Seat.DeviceId] = member
	return replaced, nil
}

func (call *Call) room(deviceId string) error {
	if len(call.members)-len(call.existing(deviceId)) >= (Limits{}).People() {
		return failures.Problems{}.CallFull()
	}
	return nil
}

func (call *Call) existing(deviceId string) []*Member {
	member, found := call.members[deviceId]
	if !found {
		return nil
	}
	return []*Member{member}
}

func (call *Call) seated(deviceId string, domainId string) (*Member, error) {
	member, found := call.members[deviceId]
	if !found || domainId != call.DomainId {
		return nil, failures.Problems{}.NotInCall()
	}
	return member, nil
}

func (call *Call) closeReplaced(replaced []*Member) {
	for _, member := range replaced {
		member.Close()
		call.context.Log.Left(member.Seat, "replaced")
	}
}

func (call *Call) remove(member *Member, reason string) {
	removed := call.removed(member)
	member.Close()
	call.logRemoval(removed, member.Seat, reason)
	call.Wake()
	call.onIdle(call)
}

func (call *Call) removed(member *Member) bool {
	call.lock.Lock()
	defer call.lock.Unlock()
	return call.deleted(member)
}

func (call *Call) deleted(member *Member) bool {
	if call.members[member.Seat.DeviceId] != member {
		return false
	}
	delete(call.members, member.Seat.DeviceId)
	return true
}

func (call *Call) logRemoval(removed bool, seat Seat, reason string) {
	if removed {
		call.context.Log.Left(seat, reason)
	}
}

func (call *Call) forward(slots []*SlotOutput, feed FeedKey, packet *rtp.Packet, frame FrameStart) {
	now := call.context.Clock.Now()
	for _, slot := range slots {
		_ = slot.Forward(feed, packet, frame, now)
	}
}

func (call *Call) sources(deviceId string) []*Member {
	call.lock.Lock()
	defer call.lock.Unlock()
	return call.existing(deviceId)
}

func (call *Call) ask(feed FeedKey, sources []*Member, allowed bool) {
	if !allowed {
		return
	}
	call.askEach(feed, sources)
}

func (*Call) askEach(feed FeedKey, sources []*Member) {
	for _, source := range sources {
		_ = source.AskKeyframe(feed)
	}
}

func (call *Call) askAll(requests []FeedKey) {
	for _, feed := range requests {
		call.RequestKeyframe(feed)
	}
}

func (call *Call) memberList() []*Member {
	call.lock.Lock()
	defer call.lock.Unlock()
	return call.listed()
}

func (call *Call) listed() []*Member {
	members := make([]*Member, 0, len(call.members))
	for _, member := range call.members {
		members = append(members, member)
	}
	return members
}

func (*Call) snapshots(members []*Member, now time.Time) []SourceSnapshot {
	snapshots := make([]SourceSnapshot, len(members))
	for index, member := range members {
		snapshots[index] = member.Snapshot(now)
	}
	return snapshots
}

func (*Call) applyAll(members []*Member, sources []SourceSnapshot, now time.Time) []FeedKey {
	requests := []FeedKey{}
	for _, member := range members {
		requests = append(requests, member.Apply(ReceiverPlanner{}.Plan(member.Viewer(), sources), now)...)
	}
	return requests
}

func (*Call) closeMembers(members []*Member) {
	for _, member := range members {
		member.Close()
	}
}
