import (
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
	err = service.checkBlobKey(domainId, envelope.RawBlobKey)
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

func (service *EnvelopeCheckService) checkBlobKey(domainId string, key string) error {
	if !strings.HasPrefix(key, domainId+"/") || strings.Contains(key, "..") {
		return &InvalidEnvelope{Field: "rawBlobKey", Reason: "the raw blob key must start with the recipient domain"}
	}
	return nil
}

func (service *EnvelopeCheckService) checkSize(size int64) error {
	if size <= 0 || size > service.MaxSize() {
		return &InvalidEnvelope{Field: "size", Reason: "the size must be between 1 byte and 64 MiB"}
	}
	return nil
}
