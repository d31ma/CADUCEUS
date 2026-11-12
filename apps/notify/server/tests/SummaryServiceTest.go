import (
	"encoding/json"
	"strings"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

// @Test(JobsTestSetup, {"partition":"tests"})
type SummaryServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SummaryServiceTest) TestRunSendsOneSummaryPerMember() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Realms(domainId)
	JobsTestSetup{}.MemberSettings(domainId, "member-1", 15, "ana@"+domainId)
	JobsTestSetup{}.Stored(setup.Notification(domainId, "member-1", "msg_1"))
	JobsTestSetup{}.Stored(setup.Notification(domainId, "member-1", "msg_2"))
	pusher := outbox.NewRecordingPusher()
	progress, err := JobsTestSetup{}.Summaries(pusher).Run(domainId, JobsTestSetup{}.Budget(25))
	sent := pusher.PushedTo("mail-out")
	payload := events.EmailSendRequested{}
	decoded := json.Unmarshal(test.firstPayload(sent), &payload)
	due, _ := setup.NotificationStore().FindDueSummaries(domainId, time.Now().Unix(), 10)
	Assert.IsTrue(err == nil && decoded == nil, "summary failed")
	Assert.Equals(1, progress.Done)
	Assert.Equals(1, len(sent))
	Assert.Equals("You have 2 unread notifications in Mesail", payload.Subject)
	Assert.Equals(0, len(due))
}

func (test *SummaryServiceTest) TestRunEndsTheWaitWhenTheSummaryIsOff() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Realms(domainId)
	JobsTestSetup{}.MemberSettings(domainId, "member-1", 0, "ana@"+domainId)
	record := JobsTestSetup{}.Stored(setup.Notification(domainId, "member-1", "msg_1"))
	pusher := outbox.NewRecordingPusher()
	progress, err := JobsTestSetup{}.Summaries(pusher).Run(domainId, JobsTestSetup{}.Budget(25))
	found := JobsTestSetup{}.Found(record)
	Assert.IsTrue(err == nil, "run failed")
	Assert.Equals(0, progress.Done)
	Assert.Equals(0, len(pusher.PushedTo("mail-out")))
	Assert.Equals("none", found.SummaryState)
	Assert.Equals(int64(0), found.SummaryDueAt)
}

func (test *SummaryServiceTest) TestRunWaitsWhenTheBudgetIsTooSmall() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Realms(domainId)
	JobsTestSetup{}.MemberSettings(domainId, "member-1", 15, "ana@"+domainId)
	record := JobsTestSetup{}.Stored(setup.Notification(domainId, "member-1", "msg_1"))
	progress, err := JobsTestSetup{}.Summaries(outbox.NewRecordingPusher()).Run(domainId, JobsTestSetup{}.Budget(2))
	Assert.IsTrue(err == nil, "run failed")
	Assert.IsTrue(progress.More, "the summary was not left for later")
	Assert.Equals("waiting", JobsTestSetup{}.Found(record).SummaryState)
}

func (test *SummaryServiceTest) TestRunSendsASummaryToAnAddressAtTheCap() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Realms(domainId)
	pusher := outbox.NewRecordingPusher()
	address := strings.Repeat("a", setup.Emails(pusher).MostAddressBytes()-len("@"+domainId)) + "@" + domainId
	JobsTestSetup{}.MemberSettings(domainId, "member-1", 15, address)
	JobsTestSetup{}.Stored(setup.Notification(domainId, "member-1", "msg_1"))
	progress, err := JobsTestSetup{}.Summaries(pusher).Run(domainId, JobsTestSetup{}.Budget(25))
	Assert.IsTrue(err == nil, "run failed")
	Assert.Equals(1, progress.Done)
	Assert.Equals(1, len(pusher.PushedTo("mail-out")))
}

func (test *SummaryServiceTest) TestRunSendsNoSummaryToAnAddressOverTheCap() {
	setup := JobsTestSetup{}
	domainId := setup.Domain()
	setup.Realms(domainId)
	pusher := outbox.NewRecordingPusher()
	address := strings.Repeat("a", setup.Emails(pusher).MostAddressBytes()+1-len("@"+domainId)) + "@" + domainId
	JobsTestSetup{}.MemberSettings(domainId, "member-1", 15, address)
	record := JobsTestSetup{}.Stored(setup.Notification(domainId, "member-1", "msg_1"))
	progress, err := JobsTestSetup{}.Summaries(pusher).Run(domainId, JobsTestSetup{}.Budget(25))
	Assert.IsTrue(err == nil, "run failed")
	Assert.Equals(0, progress.Done)
	Assert.Equals(0, len(pusher.PushedTo("mail-out")))
	Assert.Equals("none", JobsTestSetup{}.Found(record).SummaryState)
}

func (test *SummaryServiceTest) firstPayload(sent []events.Envelope) []byte {
	if len(sent) == 0 {
		return []byte("{}")
	}
	return sent[0].Payload
}
