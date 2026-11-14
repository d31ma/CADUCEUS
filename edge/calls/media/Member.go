package media

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"

	"mesail.dev/edge/calls/admission"
	"mesail.dev/edge/calls/failures"
)

type Member struct {
	Seat         Seat
	JoinedAt     time.Time
	peer         *webrtc.PeerConnection
	api          *webrtc.API
	bitrate      *BitrateHolder
	host         MemberHost
	clock        admission.Clock
	source       *SourceState
	channel      *SlotChannel
	transceivers []*webrtc.RTPTransceiver
	layout       OfferLayout
	video        []*SlotOutput
	screens      []*SlotOutput
	audio        []*SlotOutput
	answer       string
	closed       atomic.Bool
	timerLock    sync.Mutex
	timer        *time.Timer
}

func NewMember(seat Seat, parts PeerParts, host MemberHost, clock admission.Clock) *Member {
	joinedAt := clock.Now()
	return &Member{
		Seat:     seat,
		JoinedAt: joinedAt,
		peer:     parts.Peer,
		api:      parts.Api,
		bitrate:  parts.Bitrate,
		host:     host,
		clock:    clock,
		source:   NewSourceState(seat.DeviceId, joinedAt),
		channel:  &SlotChannel{},
	}
}

func (member *Member) Negotiate(offer string, connectWait time.Duration) error {
	member.listen(connectWait)
	err := member.peer.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer})
	if err != nil {
		return failures.Problems{}.InvalidOffer()
	}
	member.transceivers = member.peer.GetTransceivers()
	member.layout, err = OfferLayoutReader{}.Read(member.shapes())
	if err != nil {
		return err
	}
	err = member.attachSlots()
	if err != nil {
		return err
	}
	member.answer, err = member.answered()
	return err
}

func (member *Member) Answer() string {
	return member.answer
}

func (member *Member) Viewer() ViewerState {
	return ViewerState{
		DeviceId:   member.Seat.DeviceId,
		VideoShown: member.shown(member.video),
		AudioShown: member.shown(member.audio),
		ScreenSlot: len(member.screens) > 0,
		Bitrate:    member.bitrate.Bitrate(),
	}
}

func (member *Member) Snapshot(now time.Time) SourceSnapshot {
	return member.source.Snapshot(now)
}

func (member *Member) Apply(plan ReceiverPlan, now time.Time) []FeedKey {
	requests := member.retarget(member.video, plan.Video, now)
	requests = append(requests, member.retarget(member.screens, []FeedKey{plan.Screen}, now)...)
	member.retarget(member.audio, plan.Audio, now)
	member.channel.Publish(plan.Message)
	return requests
}

func (member *Member) Slots() []*SlotOutput {
	return slices.Concat(member.video, member.screens, member.audio)
}

func (member *Member) AskKeyframe(feed FeedKey) error {
	ssrc, found := member.source.Ssrc(feed)
	return member.picture(ssrc, found)
}

func (member *Member) AddCandidates(candidates []webrtc.ICECandidateInit) error {
	return member.added(candidates)
}

func (member *Member) Connected() bool {
	return member.peer.ConnectionState() == webrtc.PeerConnectionStateConnected
}

func (member *Member) Close() {
	member.closeOnce()
}

func (member *Member) listen(connectWait time.Duration) {
	member.peer.OnTrack(member.trackArrived)
	member.peer.OnDataChannel(member.channel.Attach)
	member.peer.OnConnectionStateChange(member.stateChanged)
	member.timerLock.Lock()
	member.timer = time.AfterFunc(connectWait, member.checkConnected)
	member.timerLock.Unlock()
}

func (member *Member) shapes() []TransceiverShape {
	shapes := make([]TransceiverShape, len(member.transceivers))
	for index, transceiver := range member.transceivers {
		shapes[index] = TransceiverShape{Kind: transceiver.Kind(), Direction: transceiver.Direction()}
	}
	return shapes
}

func (member *Member) attachSlots() error {
	var videoErr, screenErr, audioErr error
	member.video, videoErr = member.outputs(member.layout.VideoSlots, Codecs{}.VideoSlot())
	member.screens, screenErr = member.outputs(member.layout.ScreenSlot, Codecs{}.VideoSlot())
	member.audio, audioErr = member.outputs(member.layout.AudioSlots, Codecs{}.AudioSlot())
	return errors.Join(videoErr, screenErr, audioErr)
}

func (member *Member) outputs(indexes []int, kind SlotKind) ([]*SlotOutput, error) {
	slots := []*SlotOutput{}
	errs := []error{}
	for _, index := range indexes {
		slot, err := member.output(index, kind, len(slots))
		slots = append(slots, slot)
		errs = append(errs, err)
	}
	return slots, errors.Join(errs...)
}

func (member *Member) output(index int, kind SlotKind, number int) (*SlotOutput, error) {
	track, err := webrtc.NewTrackLocalStaticRTP(kind.Codec, fmt.Sprintf("%s-%d-%d", kind.Name, index, number), member.streamId())
	if err != nil {
		return nil, err
	}
	sender, err := member.api.NewRTPSender(track, member.peer.SCTP().Transport())
	if err != nil {
		return nil, err
	}
	slot := NewSlotOutput(track, kind)
	go NewFeedbackReader(sender, slot, member.host).Run()
	return slot, member.transceivers[index].SetSender(sender, track)
}

func (member *Member) answered() (string, error) {
	answer, err := member.peer.CreateAnswer(nil)
	if err != nil {
		return "", failures.Problems{}.InvalidOffer()
	}
	gathered := webrtc.GatheringCompletePromise(member.peer)
	err = member.peer.SetLocalDescription(answer)
	if err != nil {
		return "", failures.Problems{}.InvalidOffer()
	}
	<-gathered
	return member.peer.LocalDescription().SDP, nil
}

func (member *Member) trackArrived(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	kind := member.layout.KindAt(slices.IndexFunc(member.transceivers, func(transceiver *webrtc.RTPTransceiver) bool {
		return transceiver.Receiver() == receiver
	}))
	if kind == "" {
		return
	}
	member.startReading(FeedKey{DeviceId: member.Seat.DeviceId, Kind: kind, Rid: track.RID()}, track, receiver)
}

func (member *Member) startReading(feed FeedKey, track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
	member.source.Track(feed, uint32(track.SSRC()))
	go NewTrackReader(track, receiver, feed, member).Run()
}

func (member *Member) stateChanged(state webrtc.PeerConnectionState) {
	if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
		member.host.Gone(member)
	}
}

func (member *Member) checkConnected() {
	if !member.Connected() {
		member.host.Gone(member)
	}
}

func (member *Member) shown(slots []*SlotOutput) []string {
	shown := make([]string, len(slots))
	for index, slot := range slots {
		shown[index] = slot.Shown()
	}
	return shown
}

func (member *Member) retarget(slots []*SlotOutput, feeds []FeedKey, now time.Time) []FeedKey {
	requests := []FeedKey{}
	for index, slot := range slots {
		requests = append(requests, member.targeted(slot, feeds[index], now)...)
	}
	return requests
}

func (*Member) targeted(slot *SlotOutput, feed FeedKey, now time.Time) []FeedKey {
	if !slot.Target(feed, now) {
		return nil
	}
	return []FeedKey{feed}
}

func (member *Member) picture(ssrc uint32, found bool) error {
	if !found {
		return nil
	}
	return member.peer.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: ssrc}})
}

func (member *Member) added(candidates []webrtc.ICECandidateInit) error {
	for _, candidate := range candidates {
		err := member.peer.AddICECandidate(candidate)
		if err != nil {
			return failures.Problems{}.InvalidCandidate()
		}
	}
	return nil
}

func (member *Member) closeOnce() {
	if !member.closed.CompareAndSwap(false, true) {
		return
	}
	member.stopTimer()
	_ = member.peer.Close()
}

func (member *Member) stopTimer() {
	member.timerLock.Lock()
	defer member.timerLock.Unlock()
	member.timer.Stop()
}

func (*Member) streamId() string {
	return "calls"
}
