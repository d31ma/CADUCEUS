import (
	"regexp"
	"sort"
	"strings"
)

// @Service
type EnvelopeCheckService struct{}

func (service *EnvelopeCheckService) DomainOf(envelope InboundEnvelope) (string, error) {
	err := service.checkCount(envelope.Recipients)
	if err != nil {
		return "", err
	}
	domainId, err := service.oneDomain(service.domains(envelope.Recipients))
	if err != nil {
		return "", err
	}
	err = service.checkDomain(domainId)
	if err != nil {
		return "", err
	}
	err = service.checkHandoffSender(envelope)
	if err != nil {
		return "", err
	}
	err = service.checkBlobKey(service.blobOwner(envelope, domainId), envelope.RawBlobKey)
	if err != nil {
		return "", err
	}
	return domainId, service.checkSize(envelope.Size)
}

func (service *EnvelopeCheckService) MaxRecipients() int {
	return 100
}

func (service *EnvelopeCheckService) MaxSize() int64 {
	return 64 << 20
}

func (service *EnvelopeCheckService) recipientsField() string {
	return "recipients"
}

func (service *EnvelopeCheckService) checkCount(recipients []string) error {
	if len(recipients) == 0 || len(recipients) > service.MaxRecipients() {
		return &InvalidEnvelope{Field: service.recipientsField(), Reason: "between 1 and 100 recipients are required"}
	}
	return nil
}

func (service *EnvelopeCheckService) domains(recipients []string) []string {
	unique := map[string]bool{}
	for _, recipient := range recipients {
		unique[service.domainPart(recipient)] = true
	}
	return service.sortedKeys(unique)
}

func (service *EnvelopeCheckService) sortedKeys(unique map[string]bool) []string {
	keys := make([]string, 0, len(unique))
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (service *EnvelopeCheckService) domainPart(address string) string {
	return strings.ToLower(address[strings.LastIndex(address, "@")+1:])
}

func (service *EnvelopeCheckService) oneDomain(domains []string) (string, error) {
	if len(domains) != 1 {
		return "", &InvalidEnvelope{Field: service.recipientsField(), Reason: "all recipients must be in one domain"}
	}
	return domains[0], nil
}

func (service *EnvelopeCheckService) checkDomain(domainId string) error {
	if !strings.Contains(domainId, ".") || strings.ContainsAny(domainId, "/\\ @") {
		return &InvalidEnvelope{Field: service.recipientsField(), Reason: "the recipient domain is not valid"}
	}
	return nil
}

func (service *EnvelopeCheckService) SenderDomain(envelope InboundEnvelope) string {
	return service.domainPart(envelope.EnvelopeFrom)
}

func (service *EnvelopeCheckService) blobOwner(envelope InboundEnvelope, domainId string) string {
	if envelope.AuthResults.Handoff {
		return service.SenderDomain(envelope)
	}
	return domainId
}

func (service *EnvelopeCheckService) checkHandoffSender(envelope InboundEnvelope) error {
	if envelope.AuthResults.Handoff && service.domainPart(envelope.Headers.From.Address) != service.SenderDomain(envelope) {
		return &InvalidEnvelope{Field: "headers.from", Reason: "a hand-off must come from an address of the sender domain"}
	}
	return nil
}

func (service *EnvelopeCheckService) checkBlobKey(owner string, key string) error {
	if !regexp.MustCompile(service.blobKeyPattern(owner)).MatchString(key) {
		return &InvalidEnvelope{Field: "rawBlobKey", Reason: "the raw blob key must be a blob of the domain that owns it"}
	}
	return nil
}

func (service *EnvelopeCheckService) blobKeyPattern(owner string) string {
	return "^" + regexp.QuoteMeta(owner) + "/[A-Za-z0-9_-]{1,128}$"
}

func (service *EnvelopeCheckService) checkSize(size int64) error {
	if size <= 0 || size > service.MaxSize() {
		return &InvalidEnvelope{Field: "size", Reason: "the size must be between 1 byte and 64 MiB"}
	}
	return nil
}
