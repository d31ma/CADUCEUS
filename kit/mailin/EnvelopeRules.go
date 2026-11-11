package mailin

import (
	"regexp"
	"slices"
	"strings"
)

type EnvelopeRules struct{}

func (rules EnvelopeRules) Check(envelope Envelope) (string, error) {
	err := rules.checkCount(envelope.Recipients)
	if err != nil {
		return "", err
	}
	domainId, err := rules.oneDomain(envelope.Recipients)
	if err != nil {
		return "", err
	}
	err = rules.checkHandoffSender(envelope)
	if err != nil {
		return "", err
	}
	err = rules.checkBlobKey(rules.blobOwner(envelope, domainId), envelope.RawBlobKey)
	if err != nil {
		return "", err
	}
	return domainId, rules.checkSize(envelope.Size)
}

func (EnvelopeRules) MaxRecipients() int {
	return 100
}

func (EnvelopeRules) MaxSize() int64 {
	return 64 << 20
}

func (rules EnvelopeRules) DomainOf(address string) string {
	return strings.ToLower(address[strings.LastIndex(address, "@")+1:])
}

func (EnvelopeRules) recipientsField() string {
	return "recipients"
}

func (rules EnvelopeRules) checkCount(recipients []string) error {
	if len(recipients) == 0 || len(recipients) > rules.MaxRecipients() {
		return Invalid{Field: rules.recipientsField(), Reason: "between 1 and 100 recipients are required"}
	}
	return nil
}

func (rules EnvelopeRules) oneDomain(recipients []string) (string, error) {
	domains := rules.domains(recipients)
	if len(domains) != 1 {
		return "", Invalid{Field: rules.recipientsField(), Reason: "all recipients must be in one domain"}
	}
	return domains[0], rules.checkDomain(domains[0])
}

func (rules EnvelopeRules) domains(recipients []string) []string {
	domains := []string{}
	for _, recipient := range recipients {
		domains = append(domains, rules.DomainOf(recipient))
	}
	slices.Sort(domains)
	return slices.Compact(domains)
}

func (rules EnvelopeRules) checkDomain(domainId string) error {
	if !strings.Contains(domainId, ".") || strings.ContainsAny(domainId, "/\\ @") {
		return Invalid{Field: rules.recipientsField(), Reason: "the recipient domain is not valid"}
	}
	return nil
}

func (rules EnvelopeRules) blobOwner(envelope Envelope, domainId string) string {
	if envelope.AuthResults.Handoff {
		return rules.DomainOf(envelope.EnvelopeFrom)
	}
	return domainId
}

func (rules EnvelopeRules) checkHandoffSender(envelope Envelope) error {
	if envelope.AuthResults.Handoff && rules.DomainOf(envelope.Headers.From.Address) != rules.DomainOf(envelope.EnvelopeFrom) {
		return Invalid{Field: "headers.from", Reason: "a hand-off must come from an address of the sender domain"}
	}
	return nil
}

func (rules EnvelopeRules) checkBlobKey(owner string, key string) error {
	if !regexp.MustCompile(rules.blobKeyPattern(owner)).MatchString(key) {
		return Invalid{Field: "rawBlobKey", Reason: "the raw blob key must be a blob of the domain that owns it"}
	}
	return nil
}

func (EnvelopeRules) blobKeyPattern(owner string) string {
	return "^" + regexp.QuoteMeta(owner) + "/[A-Za-z0-9_-]{1,128}$"
}

func (rules EnvelopeRules) checkSize(size int64) error {
	if size <= 0 || size > rules.MaxSize() {
		return Invalid{Field: "size", Reason: "the size must be between 1 byte and 64 MiB"}
	}
	return nil
}
