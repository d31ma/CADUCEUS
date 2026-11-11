import (
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/notifications"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type FanoutWriterServiceTest struct {
	behaviors *BehaviorContext
}

func (test *FanoutWriterServiceTest) writer() *FanoutWriterService {
	return &FanoutWriterService{store: NotifyTestSetup{}.NotificationStore(), states: &NotificationStatesService{}, timestamps: &TimestampService{}}
}

func (test *FanoutWriterServiceTest) batch(domainId string, createdAt string) FanoutBatch {
	message := events.MessageCreated{MessageId: "msg_1", ConversationId: "cnv_1", AuthorKind: "member", AuthorMemberId: "member-1", AuthorDisplayName: "Ana", CreatedAt: createdAt}
	recipients := []notifications.Recipient{{MemberId: "member-2", Kind: "mention"}, {MemberId: "member-3", Kind: "mention"}, {MemberId: "member-4", Kind: "mention"}}
	settings := map[string]MemberSettingsRecord{"member-2": {MemberId: "member-2", EmailSummaryAfterMinutes: 15}}
	return FanoutBatch{DomainId: domainId, Message: message, Recipients: recipients, Settings: settings}
}

func (test *FanoutWriterServiceTest) stored(domainId string) []NotificationRecord {
	records, err := NotifyTestSetup{}.NotificationStore().FindByMessage(domainId, "msg_1", 100)
	if err != nil {
		panic(err)
	}
	return records
}

func (test *FanoutWriterServiceTest) TestWriteInsertsUpToTheBudget() {
	domainId := NotifyTestSetup{}.Domain()
	result, err := test.writer().Write(test.batch(domainId, time.Now().UTC().Format(time.RFC3339)), 2)
	Assert.IsTrue(err == nil, "write failed")
	Assert.Equals(2, result.Written)
	Assert.Equals(2, len(result.Inserted))
	Assert.Equals([]notifications.Recipient{{MemberId: "member-4", Kind: "mention"}}, result.Remaining)
	Assert.Equals(2, len(test.stored(domainId)))
}

func (test *FanoutWriterServiceTest) TestWriteSkipsMembersWhoAlreadyHaveTheNotification() {
	domainId := NotifyTestSetup{}.Domain()
	batch := test.batch(domainId, time.Now().UTC().Format(time.RFC3339))
	test.writer().Write(batch, 20)
	again, err := test.writer().Write(batch, 20)
	Assert.IsTrue(err == nil, "write failed")
	Assert.Equals(0, again.Written)
	Assert.Equals(0, len(again.Remaining))
	Assert.Equals(3, len(test.stored(domainId)))
}

func (test *FanoutWriterServiceTest) TestWriteUsesTheFixedLayoutAndTheSummaryDelay() {
	domainId := NotifyTestSetup{}.Domain()
	moment := time.Now().UTC().Truncate(time.Second)
	result, err := test.writer().Write(test.batch(domainId, moment.Format(time.RFC3339)), 20)
	Assert.IsTrue(err == nil, "write failed")
	Assert.Equals(moment.Format("2006-01-02T15:04:05.000Z"), result.Inserted[0].CreatedAt)
	Assert.Equals("pending", result.Inserted[0].PushState)
	Assert.Equals(moment.Add(15*time.Minute).Unix(), result.Inserted[0].SummaryDueAt)
	Assert.Equals("waiting", result.Inserted[0].SummaryState)
	Assert.Equals(int64(0), result.Inserted[1].SummaryDueAt)
	Assert.Equals("none", result.Inserted[1].SummaryState)
	Assert.Equals("msg_1/member-2", result.Inserted[0].DedupeKey)
	Assert.Equals("Ana", result.Inserted[0].AuthorDisplayName)
}

func (test *FanoutWriterServiceTest) TestWriteUsesNowForAnUnreadableTime() {
	domainId := NotifyTestSetup{}.Domain()
	before := time.Now().Add(-time.Second)
	result, err := test.writer().Write(test.batch(domainId, "yesterday"), 1)
	createdAt, parseFailure := (&TimestampService{}).Parse(result.Inserted[0].CreatedAt)
	Assert.IsTrue(err == nil && parseFailure == nil, "write failed")
	Assert.IsTrue(createdAt.After(before), "the time is not now")
}
