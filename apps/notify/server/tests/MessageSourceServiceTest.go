import (
	"errors"
	"net/http/httptest"
	"net/url"
)

// @Test(JobsTestSetup, {"partition":"tests"})
type MessageSourceServiceTest struct {
	behaviors *BehaviorContext
}

func (test *MessageSourceServiceTest) source(authorization string, messageId string, domainId string) (MessageSource, error) {
	request := httptest.NewRequest("GET", "/api/v1/notify/messages/"+messageId+"/source?"+url.Values{"domainId": {domainId}}.Encode(), nil)
	return JobsTestSetup{}.Sources().Requested(authorization, messageId, request)
}

func (test *MessageSourceServiceTest) TestSourceGivesTheSummaryBody() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	member, _ := setup.Realms(domainId)
	stored, _ := setup.SummaryStore().InsertIfAbsent(setup.StagedSummary(domainId))
	source, err := test.source(setup.Service(member), "sum_"+stored.Id, "")
	Assert.IsTrue(err == nil, "source failed")
	Assert.Equals("sum_"+stored.Id, source.MessageId)
	Assert.Equals(domainId, source.DomainId)
	Assert.Equals("1 unread", source.Body)
	Assert.Equals([]string{}, source.AttachmentIds)
}

func (test *MessageSourceServiceTest) TestSourceRefusesAnotherDomain() {
	setup := JobsTestSetup{}
	stored, _ := setup.SummaryStore().InsertIfAbsent(setup.StagedSummary(setup.Domain()))
	other, _ := setup.Realms(setup.Domain())
	_, err := test.source(setup.Service(other), "sum_"+stored.Id, "")
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
	Assert.Equals(4511, missing.Number())
}

func (test *MessageSourceServiceTest) TestSourceIgnoresTheDomainOfADomainService() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	stored, _ := setup.SummaryStore().InsertIfAbsent(setup.StagedSummary(domainId))
	other, _ := setup.Realms(setup.Domain())
	_, err := test.source(setup.Service(other), "sum_"+stored.Id, domainId)
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
	Assert.Equals(4511, missing.Number())
}

func (test *MessageSourceServiceTest) TestSourceLetsAnOperatorNameTheDomain() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	_, installWide := setup.Realms(domainId)
	stored, _ := setup.SummaryStore().InsertIfAbsent(setup.StagedSummary(domainId))
	source, err := test.source(setup.Service(installWide), "sum_"+stored.Id, domainId)
	Assert.IsTrue(err == nil, "source failed")
	Assert.Equals(domainId, source.DomainId)
}

func (test *MessageSourceServiceTest) TestSourceRefusesAnOperatorWithoutADomain() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	_, installWide := setup.Realms(domainId)
	stored, _ := setup.SummaryStore().InsertIfAbsent(setup.StagedSummary(domainId))
	_, err := test.source(setup.Service(installWide), "sum_"+stored.Id, "")
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
	Assert.Equals(4511, missing.Number())
}

func (test *MessageSourceServiceTest) TestSourceRefusesAnotherPrefix() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	member, _ := setup.Realms(domainId)
	stored, _ := setup.SummaryStore().InsertIfAbsent(setup.StagedSummary(domainId))
	_, err := test.source(setup.Service(member), "msg_"+stored.Id, "")
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
	Assert.Equals(4511, missing.Number())
}

func (test *MessageSourceServiceTest) TestSourceRefusesABadToken() {
	setup := JobsTestSetup{}
	setup.Realms(setup.Domain())
	_, err := test.source("Bearer nonsense", "sum_1", "")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused))
	Assert.Equals(4501, refused.Number())
}
