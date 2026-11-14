package desk

import (
	"testing"

	"github.com/pion/webrtc/v4"

	"mesail.dev/edge/calls/failures"
)

func TestCandidateListsMustHoldOneToTwenty(t *testing.T) {
	cases := [][]webrtc.ICECandidateInit{nil, make([]webrtc.ICECandidateInit, 21)}
	for _, candidates := range cases {
		err := (CallDesk{}).withinLimit(candidates)
		if (failures.Problems{}).From(err).Number() != 6510 {
			t.Fatalf("%d candidates gave %v", len(candidates), err)
		}
	}
	if (CallDesk{}).withinLimit(make([]webrtc.ICECandidateInit, 20)) != nil {
		t.Fatal("20 candidates were refused")
	}
}
