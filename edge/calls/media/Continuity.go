package media

import "github.com/pion/rtp"

type Continuity struct {
	step            uint32
	switched        bool
	lastSequence    uint16
	lastTimestamp   uint32
	sequenceOffset  uint16
	timestampOffset uint32
}

func NewContinuity(step uint32) *Continuity {
	return &Continuity{step: step, switched: true}
}

func (continuity *Continuity) Restart() {
	continuity.switched = true
}

func (continuity *Continuity) Rewrite(header *rtp.Header) {
	continuity.align(header)
	header.SequenceNumber += continuity.sequenceOffset
	header.Timestamp += continuity.timestampOffset
	continuity.lastSequence = header.SequenceNumber
	continuity.lastTimestamp = header.Timestamp
}

func (continuity *Continuity) align(header *rtp.Header) {
	if !continuity.switched {
		return
	}
	continuity.switched = false
	continuity.sequenceOffset = continuity.lastSequence + 1 - header.SequenceNumber
	continuity.timestampOffset = continuity.lastTimestamp + continuity.step - header.Timestamp
}
