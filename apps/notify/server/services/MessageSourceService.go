import (
	"cmp"

	"mesail.dev/packages/ids"
	"mesail.dev/packages/oidc"
)

// @Service
type MessageSourceService struct {
	// @Needs
	callers *CallerService
	// @Needs
	summaries *SummaryStoreService
}

func (service *MessageSourceService) Source(authorization string, messageId string, requestedDomainId string) (MessageSource, error) {
	caller, failure := service.callers.Caller(authorization)
	storedId, invalidId := ids.WireId{}.Strip(service.prefix(), messageId)
	domainId := service.domainFor(caller, requestedDomainId)
	record, found, failure := service.findUnless(cmp.Or(failure, service.missing(invalidId)), domainId, storedId)
	return service.source(record), cmp.Or(failure, service.matching(record, found, domainId))
}

func (service *MessageSourceService) domainFor(caller oidc.Caller, requestedDomainId string) string {
	if service.callers.IsInstallWide(caller.DomainId) {
		return requestedDomainId
	}
	return caller.DomainId
}

func (service *MessageSourceService) missing(invalidId error) error {
	if invalidId != nil {
		return service.missingMessage()
	}
	return nil
}

func (service *MessageSourceService) findUnless(failure error, domainId string, storedId string) (SummaryRecord, bool, error) {
	if failure != nil || domainId == "" {
		return SummaryRecord{}, false, failure
	}
	return service.summaries.Find(domainId, storedId)
}

func (service *MessageSourceService) matching(record SummaryRecord, found bool, domainId string) error {
	if !found || record.DomainId != domainId {
		return service.missingMessage()
	}
	return nil
}

func (service *MessageSourceService) source(record SummaryRecord) MessageSource {
	return MessageSource{
		MessageId:     ids.WireId{}.Add(service.prefix(), record.Id),
		DomainId:      record.DomainId,
		Body:          record.Body,
		AttachmentIds: []string{},
	}
}

func (service *MessageSourceService) missingMessage() error {
	return &NotFound{What: "message"}
}

func (service *MessageSourceService) prefix() string {
	return "sum"
}
