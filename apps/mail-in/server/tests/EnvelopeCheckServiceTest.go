import "errors"

// @Test(InboundTestSetup, {"partition":"tests"})
type EnvelopeCheckServiceTest struct {
	behaviors *BehaviorContext
}

func (test *EnvelopeCheckServiceTest) envelope(recipients []string, key string, size int64) InboundEnvelope {
	return InboundEnvelope{EnvelopeFrom: "dan@outside.test", Recipients: recipients, RawBlobKey: key, Size: size}
}

func (test *EnvelopeCheckServiceTest) invalidField(err error) string {
	invalid := &InvalidEnvelope{}
	errors.As(err, &invalid)
	return invalid.Field
}

func (test *EnvelopeCheckServiceTest) TestDomainOfNamesTheRecipientDomain() {
	domainId, err := (&EnvelopeCheckService{}).DomainOf(test.envelope([]string{"ana@Example.COM", "bo@example.com"}, "example.com/inbound/a.eml", 10))
	Assert.IsTrue(err == nil)
	Assert.Equals("example.com", domainId)
}

func (test *EnvelopeCheckServiceTest) TestDomainOfRefusesNoRecipients() {
	_, err := (&EnvelopeCheckService{}).DomainOf(test.envelope([]string{}, "example.com/inbound/a.eml", 10))
	Assert.Equals("recipients", test.invalidField(err))
}

func (test *EnvelopeCheckServiceTest) TestDomainOfRefusesTooManyRecipients() {
	recipients := make([]string, 101)
	_, err := (&EnvelopeCheckService{}).DomainOf(test.envelope(recipients, "example.com/inbound/a.eml", 10))
	Assert.Equals("recipients", test.invalidField(err))
}

func (test *EnvelopeCheckServiceTest) TestDomainOfRefusesTwoDomains() {
	_, err := (&EnvelopeCheckService{}).DomainOf(test.envelope([]string{"ana@example.com", "eve@example.org"}, "example.com/inbound/a.eml", 10))
	Assert.Equals("recipients", test.invalidField(err))
}

func (test *EnvelopeCheckServiceTest) TestDomainOfRefusesADomainWithoutADot() {
	_, err := (&EnvelopeCheckService{}).DomainOf(test.envelope([]string{"root@localhost"}, "localhost/inbound/a.eml", 10))
	Assert.Equals("recipients", test.invalidField(err))
}

func (test *EnvelopeCheckServiceTest) TestDomainOfRefusesAKeyOfAnotherDomain() {
	_, err := (&EnvelopeCheckService{}).DomainOf(test.envelope([]string{"ana@example.com"}, "example.org/inbound/a.eml", 10))
	Assert.Equals("rawBlobKey", test.invalidField(err))
}

func (test *EnvelopeCheckServiceTest) TestDomainOfRefusesAKeyThatClimbsOut() {
	_, err := (&EnvelopeCheckService{}).DomainOf(test.envelope([]string{"ana@example.com"}, "example.com/../example.org/a.eml", 10))
	Assert.Equals("rawBlobKey", test.invalidField(err))
}

func (test *EnvelopeCheckServiceTest) TestDomainOfRefusesAnEmptyMessage() {
	_, err := (&EnvelopeCheckService{}).DomainOf(test.envelope([]string{"ana@example.com"}, "example.com/inbound/a.eml", 0))
	Assert.Equals("size", test.invalidField(err))
}

func (test *EnvelopeCheckServiceTest) TestDomainOfRefusesATooLargeMessage() {
	service := &EnvelopeCheckService{}
	_, err := service.DomainOf(test.envelope([]string{"ana@example.com"}, "example.com/inbound/a.eml", service.MaxSize()+1))
	Assert.Equals("size", test.invalidField(err))
}

func (test *EnvelopeCheckServiceTest) TestMaxRecipientsIsOneHundred() {
	Assert.Equals(100, (&EnvelopeCheckService{}).MaxRecipients())
}

func (test *EnvelopeCheckServiceTest) TestMaxSizeIsSixtyFourMebibytes() {
	Assert.Equals(int64(67108864), (&EnvelopeCheckService{}).MaxSize())
}
