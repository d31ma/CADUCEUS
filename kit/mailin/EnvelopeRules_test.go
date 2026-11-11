package mailin

import "testing"

func ruleEnvelope(recipients []string, key string, size int64) Envelope {
	return Envelope{EnvelopeFrom: "dan@outside.test", Recipients: recipients, RawBlobKey: key, Size: size}
}

func handoff(key string, from string) Envelope {
	envelope := ruleEnvelope([]string{"eve@example.org"}, key, 10)
	envelope.EnvelopeFrom = "ana@example.com"
	envelope.Headers.From = Mailbox{Address: from}
	envelope.AuthResults.Handoff = true
	return envelope
}

func checkedField(envelope Envelope) string {
	_, err := EnvelopeRules{}.Check(envelope)
	return invalidField(err)
}

func TestCheckNamesTheRecipientDomain(t *testing.T) {
	domainId, err := EnvelopeRules{}.Check(ruleEnvelope([]string{"ana@Example.COM", "bo@example.com"}, "example.com/inbound-a", 10))
	check(t, err == nil && domainId == "example.com", "want example.com")
}

func TestCheckRefusesNoRecipients(t *testing.T) {
	check(t, checkedField(ruleEnvelope([]string{}, "example.com/inbound-a", 10)) == "recipients", "want recipients")
}

func TestCheckRefusesTooManyRecipients(t *testing.T) {
	check(t, checkedField(ruleEnvelope(make([]string, 101), "example.com/inbound-a", 10)) == "recipients", "want recipients")
}

func TestCheckRefusesTwoDomains(t *testing.T) {
	check(t, checkedField(ruleEnvelope([]string{"ana@example.com", "eve@example.org"}, "example.com/inbound-a", 10)) == "recipients", "want recipients")
}

func TestCheckRefusesADomainWithoutADot(t *testing.T) {
	check(t, checkedField(ruleEnvelope([]string{"root@localhost"}, "localhost/inbound-a", 10)) == "recipients", "want recipients")
}

func TestCheckRefusesADomainWithASlash(t *testing.T) {
	check(t, checkedField(ruleEnvelope([]string{"ana@a/b.com"}, "a/b.com/inbound-a", 10)) == "recipients", "want recipients")
}

func TestCheckRefusesAKeyOfAnotherDomain(t *testing.T) {
	check(t, checkedField(ruleEnvelope([]string{"ana@example.com"}, "example.org/inbound-a", 10)) == "rawBlobKey", "want rawBlobKey")
}

func TestCheckRefusesAKeyThatClimbsOut(t *testing.T) {
	check(t, checkedField(ruleEnvelope([]string{"ana@example.com"}, "example.com/../example.org/a.eml", 10)) == "rawBlobKey", "want rawBlobKey")
}

func TestCheckRefusesAKeyWithAFolder(t *testing.T) {
	check(t, checkedField(ruleEnvelope([]string{"ana@example.com"}, "example.com/inbound/a.eml", 10)) == "rawBlobKey", "want rawBlobKey")
}

func TestCheckRefusesAnEmptyMessage(t *testing.T) {
	check(t, checkedField(ruleEnvelope([]string{"ana@example.com"}, "example.com/inbound-a", 0)) == "size", "want size")
}

func TestCheckRefusesATooLargeMessage(t *testing.T) {
	check(t, checkedField(ruleEnvelope([]string{"ana@example.com"}, "example.com/inbound-a", EnvelopeRules{}.MaxSize()+1)) == "size", "want size")
}

func TestCheckAcceptsTheLargestMessage(t *testing.T) {
	_, err := EnvelopeRules{}.Check(ruleEnvelope([]string{"ana@example.com"}, "example.com/inbound-a", EnvelopeRules{}.MaxSize()))
	check(t, err == nil, "want the largest size accepted")
}

func TestCheckAcceptsAHandoffBlobOfTheSenderDomain(t *testing.T) {
	domainId, err := EnvelopeRules{}.Check(handoff("example.com/outbound-1", "ana@example.com"))
	check(t, err == nil && domainId == "example.org", "want example.org")
}

func TestCheckRefusesAHandoffBlobOfTheRecipientDomain(t *testing.T) {
	check(t, checkedField(handoff("example.org/outbound-1", "ana@example.com")) == "rawBlobKey", "want rawBlobKey")
}

func TestCheckRefusesAHandoffFromAnotherDomain(t *testing.T) {
	check(t, checkedField(handoff("example.com/outbound-1", "mallory@example.net")) == "headers.from", "want headers.from")
}

func TestTheLimitsAreOneHundredRecipientsAndSixtyFourMebibytes(t *testing.T) {
	check(t, EnvelopeRules{}.MaxRecipients() == 100, "want 100")
	check(t, EnvelopeRules{}.MaxSize() == 67108864, "want 64 MiB")
}

func TestDomainOfIsTheLowerCaseDomain(t *testing.T) {
	check(t, EnvelopeRules{}.DomainOf("Ana@Example.com") == "example.com", "want example.com")
	check(t, Invalid{Field: "size", Reason: "too large"}.Error() == "size: too large", "wrong text")
}
