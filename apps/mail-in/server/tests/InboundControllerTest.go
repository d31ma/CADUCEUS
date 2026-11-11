import "errors"

// @Test(InboundTestSetup, {"partition":"tests"})
type InboundControllerTest struct {
	behaviors *BehaviorContext
}

func (test *InboundControllerTest) TestPostMessageRefusesAnEnvelopeWithoutRecipients() {
	controller := &InboundController{inboundService: &InboundService{envelopeCheckService: &EnvelopeCheckService{}}}
	_, err := controller.PostMessage(InboundEnvelope{RawBlobKey: "example.com/inbound/x.eml", Size: 1})
	invalid := &InvalidEnvelope{}
	Assert.IsTrue(errors.As(err, &invalid))
}
