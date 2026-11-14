package media

import (
	"sync"

	"github.com/pion/webrtc/v4"

	"mesail.dev/edge/calls/failures"
)

type Calls struct {
	opener  MemberOpener
	context CallContext
	lock    sync.Mutex
	calls   map[string]*Call
}

func NewCalls(opener MemberOpener, context CallContext) *Calls {
	return &Calls{opener: opener, context: context, calls: map[string]*Call{}}
}

func (calls *Calls) Join(seat Seat, offer string) (string, error) {
	call, err := calls.reserve(seat)
	if err != nil {
		return "", err
	}
	member, err := calls.opener.Open(seat, offer, call)
	if err != nil {
		return "", calls.abandoned(call, err)
	}
	err = call.Add(member)
	if err != nil {
		return "", calls.refused(call, member, err)
	}
	calls.context.Log.Joined(seat, call.Size())
	return member.Answer(), nil
}

func (calls *Calls) AddCandidates(seat Seat, candidates []webrtc.ICECandidateInit) error {
	_, member, err := calls.seated(seat)
	if err != nil {
		return err
	}
	return member.AddCandidates(candidates)
}

func (calls *Calls) Leave(seat Seat) error {
	call, member, err := calls.seated(seat)
	if err != nil {
		return err
	}
	call.Leave(member)
	return nil
}

func (calls *Calls) Count() int {
	calls.lock.Lock()
	defer calls.lock.Unlock()
	return len(calls.calls)
}

func (calls *Calls) Size(callId string) int {
	return calls.sizeOf(calls.find(callId))
}

func (*Calls) sizeOf(call *Call, err error) int {
	if err != nil {
		return 0
	}
	return call.Size()
}

func (calls *Calls) Close() {
	calls.closeAll(calls.list())
}

func (calls *Calls) reserve(seat Seat) (*Call, error) {
	calls.lock.Lock()
	defer calls.lock.Unlock()
	call := calls.existingOrNew(seat)
	return call, call.Reserve(seat.DomainId)
}

func (calls *Calls) existingOrNew(seat Seat) *Call {
	call, found := calls.calls[seat.CallId]
	if found {
		return call
	}
	call = NewCall(seat, calls.context, calls.dropIfIdle)
	calls.calls[seat.CallId] = call
	call.Start()
	return call
}

func (calls *Calls) abandoned(call *Call, err error) error {
	call.Abandon()
	calls.dropIfIdle(call)
	return err
}

func (calls *Calls) refused(call *Call, member *Member, err error) error {
	member.Close()
	calls.dropIfIdle(call)
	return err
}

func (calls *Calls) dropIfIdle(call *Call) {
	calls.lock.Lock()
	defer calls.lock.Unlock()
	calls.drop(call)
}

func (calls *Calls) drop(call *Call) {
	if calls.calls[call.Id] != call || !call.Idle() {
		return
	}
	delete(calls.calls, call.Id)
	call.Stop()
	calls.context.Log.Ended(call.Id)
}

func (calls *Calls) seated(seat Seat) (*Call, *Member, error) {
	call, err := calls.find(seat.CallId)
	if err != nil {
		return nil, nil, err
	}
	member, err := call.Member(seat.DeviceId, seat.DomainId)
	return call, member, err
}

func (calls *Calls) find(callId string) (*Call, error) {
	calls.lock.Lock()
	defer calls.lock.Unlock()
	return calls.found(callId)
}

func (calls *Calls) found(callId string) (*Call, error) {
	call, found := calls.calls[callId]
	if !found {
		return nil, failures.Problems{}.NotInCall()
	}
	return call, nil
}

func (calls *Calls) list() []*Call {
	calls.lock.Lock()
	defer calls.lock.Unlock()
	return calls.listed()
}

func (calls *Calls) listed() []*Call {
	list := make([]*Call, 0, len(calls.calls))
	for _, call := range calls.calls {
		list = append(list, call)
	}
	return list
}

func (*Calls) closeAll(list []*Call) {
	for _, call := range list {
		call.Stop()
		call.CloseAll()
	}
}
