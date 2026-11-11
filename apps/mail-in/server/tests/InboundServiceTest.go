import (
	"crypto/rand"
	"errors"
)

// @Test(InboundTestSetup, {"partition":"tests"})
type InboundServiceTest struct {
	behaviors *BehaviorContext
}

func (test *InboundServiceTest) service() *InboundService {
	return &InboundService{
		envelopeCheckService:   &EnvelopeCheckService{},
		inboundDocumentService: &InboundDocumentService{},
		inboundRawService:      &InboundRawService{},
	}
}

func (test *InboundServiceTest) envelope(key string) InboundEnvelope {
	return InboundEnvelope{
		EnvelopeFrom: "dan@outside.test",
		Recipients:   []string{"ana@example.com"},
		RawBlobKey:   key,
		Size:         64,
		Headers:      InboundHeaders{MessageId: "bb-1@outside.test", Subject: "Hello"},
		AuthResults:  AuthResults{Spf: "pass", Dkim: "pass", Dmarc: "pass", DmarcPolicy: "none"},
	}
}

func (test *InboundServiceTest) freshKey() string {
	return "example.com/inbound-" + rand.Text()
}

func (test *InboundServiceTest) TestAcceptRefusesAnInvalidEnvelope() {
	_, err := test.service().Accept(test.envelope("example.org/inbound-x"))
	invalid := &InvalidEnvelope{}
	Assert.IsTrue(errors.As(err, &invalid))
}

func (test *InboundServiceTest) TestAcceptStoresTheMessageOnce() {
	service := test.service()
	key := test.freshKey()
	first, err := service.Accept(test.envelope(key))
	second, again := service.Accept(test.envelope(key))
	Assert.IsTrue(err == nil, "first accept failed")
	Assert.IsTrue(again == nil, "second accept failed")
	Assert.Equals(first.InboundId, second.InboundId)
}
