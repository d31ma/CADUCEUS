package media

import "github.com/pion/webrtc/v4"

type FeedbackReader struct {
	sender *webrtc.RTPSender
	slot   *SlotOutput
	host   MemberHost
}

func NewFeedbackReader(sender *webrtc.RTPSender, slot *SlotOutput, host MemberHost) FeedbackReader {
	return FeedbackReader{sender: sender, slot: slot, host: host}
}

func (reader FeedbackReader) Run() {
	reader.loop()
}

func (reader FeedbackReader) loop() {
	for reader.next() {
	}
}

func (reader FeedbackReader) next() bool {
	packets, _, err := reader.sender.ReadRTCP()
	if err != nil {
		return false
	}
	reader.answer(KeyframeRequests{}.Asked(packets))
	return true
}

func (reader FeedbackReader) answer(asked bool) {
	if asked {
		reader.host.RequestKeyframe(reader.slot.Current())
	}
}
