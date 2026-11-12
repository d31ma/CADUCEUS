import (
	"errors"
	"net/http/httptest"
)

// @Test(JobsTestSetup, {"partition":"tests"})
type MessageSourceControllerTest struct {
	behaviors *BehaviorContext
}

func (test *MessageSourceControllerTest) controller() *MessageSourceController {
	return &MessageSourceController{sources: JobsTestSetup{}.Sources()}
}

func (test *MessageSourceControllerTest) TestGetSourceAnswersTheSummary() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	member, _ := setup.Realms(domainId)
	stored, _ := setup.SummaryStore().InsertIfAbsent(setup.StagedSummary(domainId))
	request := httptest.NewRequest("GET", "/api/v1/notify/messages/sum_"+stored.Id+"/source", nil)
	source, err := test.controller().GetSource(setup.Service(member), "sum_"+stored.Id, request)
	Assert.IsTrue(err == nil, "source failed")
	Assert.Equals("1 unread", source.Body)
}

func (test *MessageSourceControllerTest) TestGetSourceReadsTheOperatorDomain() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	_, installWide := setup.Realms(domainId)
	stored, _ := setup.SummaryStore().InsertIfAbsent(setup.StagedSummary(domainId))
	request := httptest.NewRequest("GET", "/api/v1/notify/messages/sum_"+stored.Id+"/source?domainId="+domainId, nil)
	source, err := test.controller().GetSource(setup.Service(installWide), "sum_"+stored.Id, request)
	Assert.IsTrue(err == nil, "source failed")
	Assert.Equals(domainId, source.DomainId)
}

func (test *MessageSourceControllerTest) TestGetSourceHidesAnotherDomain() {
	setup := JobsTestSetup{}
	stored, _ := setup.SummaryStore().InsertIfAbsent(setup.StagedSummary(setup.Domain()))
	other, _ := setup.Realms(setup.Domain())
	request := httptest.NewRequest("GET", "/api/v1/notify/messages/sum_"+stored.Id+"/source", nil)
	_, err := test.controller().GetSource(setup.Service(other), "sum_"+stored.Id, request)
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
}
