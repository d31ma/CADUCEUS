package desk

import (
	"github.com/pion/webrtc/v4"

	"mesail.dev/edge/calls/admission"
	"mesail.dev/edge/calls/failures"
	"mesail.dev/edge/calls/media"
)

type CallDesk struct {
	check admission.JoinTokenCheck
	calls *media.Calls
}

func NewCallDesk(check admission.JoinTokenCheck, calls *media.Calls) CallDesk {
	return CallDesk{check: check, calls: calls}
}

func (desk CallDesk) Offer(request Request) (CallAnswer, error) {
	claims, err := desk.check.Admit(request.Token, request.CallId)
	if err != nil {
		return CallAnswer{}, err
	}
	offer := CallOffer{}
	err = request.Decode(&offer)
	if err != nil {
		return CallAnswer{}, err
	}
	sdp, err := desk.calls.Join(desk.seat(claims), offer.Sdp)
	return CallAnswer{Sdp: sdp}, err
}

func (desk CallDesk) AddCandidates(request Request) error {
	claims, err := desk.check.Identify(request.Token, request.CallId)
	if err != nil {
		return err
	}
	candidates := CallCandidates{}
	err = request.Decode(&candidates)
	if err != nil {
		return err
	}
	err = desk.withinLimit(candidates.Candidates)
	if err != nil {
		return err
	}
	return desk.calls.AddCandidates(desk.seat(claims), candidates.Candidates)
}

func (desk CallDesk) Leave(request Request) error {
	claims, err := desk.check.Identify(request.Token, request.CallId)
	if err != nil {
		return err
	}
	return desk.calls.Leave(desk.seat(claims))
}

func (CallDesk) seat(claims admission.JoinClaims) media.Seat {
	return media.Seat{CallId: claims.CallId, DeviceId: claims.DeviceId, DomainId: claims.DomainId}
}

func (desk CallDesk) withinLimit(candidates []webrtc.ICECandidateInit) error {
	if len(candidates) == 0 || len(candidates) > desk.maximumCandidates() {
		return failures.Problems{}.InvalidCandidate()
	}
	return nil
}

func (CallDesk) maximumCandidates() int {
	return 20
}
