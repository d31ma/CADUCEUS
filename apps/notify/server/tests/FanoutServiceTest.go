import (
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/notifications"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type FanoutServiceTest struct {
	behaviors *BehaviorContext
}

func (test *FanoutServiceTest) queue() *FanoutQueueService {
	return &FanoutQueueService{store: NotifyTestSetup{}.FanoutStore(), timestamps: &TimestampService{}}
}

func (test *FanoutServiceTest) service() *FanoutService {
	writer := &FanoutWriterService{store: NotifyTestSetup{}.NotificationStore(), states: &NotificationStatesService{}, timestamps: &TimestampService{}}
	return &FanoutService{queue: test.queue(), recipients: &RecipientService{settings: NotifyTestSetup{}.SettingsStore()}, writer: writer}
}

func (test *FanoutServiceTest) spilled(domainId string) {
	message := events.MessageCreated{MessageId: "msg_1", ConversationId: "cnv_1", ConversationKind: "direct", AuthorKind: "member", AuthorMemberId: "member-1", MemberIds: []string{"member-1", "member-2", "member-3", "member-4", "member-5"}, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	recipients := []notifications.Recipient{{MemberId: "member-2", Kind: "dm"}, {MemberId: "member-3", Kind: "dm"}, {MemberId: "member-4", Kind: "dm"}, {MemberId: "member-5", Kind: "dm"}}
	if err := test.queue().Spill(FanoutBatch{DomainId: domainId, Message: message}, recipients); err != nil {
		panic(err)
	}
}

func (test *FanoutServiceTest) stored(domainId string) []NotificationRecord {
	records, err := NotifyTestSetup{}.NotificationStore().FindByMessage(domainId, "msg_1", 100)
	if err != nil {
		panic(err)
	}
	return records
}

func (test *FanoutServiceTest) TestProcessFinishesASpilledFanout() {
	domainId := NotifyTestSetup{}.Domain()
	test.spilled(domainId)
	progress, err := test.service().Process(domainId, 25)
	waiting, _ := test.queue().Waiting(domainId, 10)
	Assert.IsTrue(err == nil, "process failed")
	Assert.Equals(JobProgress{Written: 5, Done: 1, More: false}, progress)
	Assert.Equals(4, len(test.stored(domainId)))
	Assert.Equals(0, len(waiting))
}

func (test *FanoutServiceTest) TestProcessStopsAtTheWriteBudget() {
	domainId := NotifyTestSetup{}.Domain()
	test.spilled(domainId)
	first, err := test.service().Process(domainId, 3)
	second, again := test.service().Process(domainId, 25)
	Assert.IsTrue(err == nil && again == nil, "process failed")
	Assert.Equals(JobProgress{Written: 3, Done: 0, More: true}, first)
	Assert.Equals(JobProgress{Written: 3, Done: 1, More: false}, second)
	Assert.Equals(4, len(test.stored(domainId)))
}

func (test *FanoutServiceTest) TestProcessKeepsToTheGivenDomain() {
	domainId := NotifyTestSetup{}.Domain()
	test.spilled(domainId)
	progress, err := test.service().Process(NotifyTestSetup{}.Domain(), 25)
	Assert.IsTrue(err == nil, "process failed")
	Assert.Equals(JobProgress{}, progress)
	Assert.Equals(0, len(test.stored(domainId)))
}
