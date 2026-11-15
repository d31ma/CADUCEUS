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

func checkedCase(envelope Envelope) Invalid {
	_, err := EnvelopeRules{}.Check(envelope)
	return invalidCase(err)
}

func TestCheckNamesTheRecipientDomain(t *testing.T) {
	domainId, err := EnvelopeRules{}.Check(ruleEnvelope([]string{"ana@Example.COM", "bo@example.com"}, "example.com/inbound-a", 10))
	check(t, err == nil && domainId == "example.com", "want example.com")
}

func TestCheckRefusesNoRecipients(t *testing.T) {
	check(t, checkedCase(ruleEnvelope([]string{}, "example.com/inbound-a", 10)) == Invalid{}.RecipientCount(), "want Invalid{}.RecipientCount")
}

func TestCheckRefusesTooManyRecipients(t *testing.T) {
	check(t, checkedCase(ruleEnvelope(make([]string, 101), "example.com/inbound-a", 10)) == Invalid{}.RecipientCount(), "want Invalid{}.RecipientCount")
}

func TestCheckRefusesTwoDomains(t *testing.T) {
	check(t, checkedCase(ruleEnvelope([]string{"ana@example.com", "eve@example.org"}, "example.com/inbound-a", 10)) == Invalid{}.RecipientsInManyDomains(), "want Invalid{}.RecipientsInManyDomains")
}

func TestCheckRefusesADomainWithoutADot(t *testing.T) {
	check(t, checkedCase(ruleEnvelope([]string{"root@localhost"}, "localhost/inbound-a", 10)) == Invalid{}.RecipientDomain(), "want Invalid{}.RecipientDomain")
}

func TestCheckRefusesADomainWithASlash(t *testing.T) {
	check(t, checkedCase(ruleEnvelope([]string{"ana@a/b.com"}, "a/b.com/inbound-a", 10)) == Invalid{}.RecipientDomain(), "want Invalid{}.RecipientDomain")
}

func TestCheckRefusesAKeyOfAnotherDomain(t *testing.T) {
	check(t, checkedCase(ruleEnvelope([]string{"ana@example.com"}, "example.org/inbound-a", 10)) == Invalid{}.BlobKeyOwner(), "want Invalid{}.BlobKeyOwner")
}

func TestCheckRefusesAKeyThatClimbsOut(t *testing.T) {
	check(t, checkedCase(ruleEnvelope([]string{"ana@example.com"}, "example.com/../example.org/a.eml", 10)) == Invalid{}.BlobKeyOwner(), "want Invalid{}.BlobKeyOwner")
}

func TestCheckRefusesAKeyWithAFolder(t *testing.T) {
	check(t, checkedCase(ruleEnvelope([]string{"ana@example.com"}, "example.com/inbound/a.eml", 10)) == Invalid{}.BlobKeyOwner(), "want Invalid{}.BlobKeyOwner")
}

func TestCheckRefusesAnEmptyMessage(t *testing.T) {
	check(t, checkedCase(ruleEnvelope([]string{"ana@example.com"}, "example.com/inbound-a", 0)) == Invalid{}.Size(), "want Invalid{}.Size")
}

func TestCheckRefusesATooLargeMessage(t *testing.T) {
	check(t, checkedCase(ruleEnvelope([]string{"ana@example.com"}, "example.com/inbound-a", EnvelopeRules{}.MaxSize()+1)) == Invalid{}.Size(), "want Invalid{}.Size")
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
	check(t, checkedCase(handoff("example.org/outbound-1", "ana@example.com")) == Invalid{}.BlobKeyOwner(), "want Invalid{}.BlobKeyOwner")
}

func TestCheckRefusesAHandoffFromAnotherDomain(t *testing.T) {
	check(t, checkedCase(handoff("example.com/outbound-1", "mallory@example.net")) == Invalid{}.HandOffSender(), "want Invalid{}.HandOffSender")
}

func TestTheLimitsAreOneHundredRecipientsAndSixtyFourMebibytes(t *testing.T) {
	check(t, EnvelopeRules{}.MaxRecipients() == 100, "want 100")
	check(t, EnvelopeRules{}.MaxSize() == 67108864, "want 64 MiB")
}

func TestDomainOfIsTheLowerCaseDomain(t *testing.T) {
	check(t, EnvelopeRules{}.DomainOf("Ana@Example.com") == "example.com", "want example.com")
	check(t, Invalid{Field: "size", Reason: "too large"}.Error() == "size: too large", "wrong text")
}

func TestEveryInvalidCaseHasItsOwnMailInNumber(t *testing.T) {
	check(t, Invalid{}.RecipientCount().Number() == 2501, "want 2501")
	check(t, Invalid{}.RecipientsInManyDomains().Number() == 2502, "want 2502")
	check(t, Invalid{}.RecipientDomain().Number() == 2503, "want 2503")
	check(t, Invalid{}.HandOffSender().Number() == 2504, "want 2504")
	check(t, Invalid{}.BlobKeyOwner().Number() == 2505, "want 2505")
	check(t, Invalid{}.Size().Number() == 2506, "want 2506")
	check(t, Invalid{}.SenderDomainNotHosted().Number() == 2507, "want 2507")
	check(t, Invalid{}.NoRawMessage().Number() == 2508, "want 2508")
}

func TestEveryMissingCaseHasItsOwnMailInNumber(t *testing.T) {
	check(t, Missing{}.NoOutboxCollection().Number() == 2510, "want 2510")
	check(t, Missing{}.NoInboundMessage().Number() == 2511, "want 2511")
	check(t, Missing{}.NoDeadLetter("gone", 15).Number() == 15, "want the kit number")
	check(t, Missing{}.NoOutboxDocument().Number() == 2513, "want 2513")
}
