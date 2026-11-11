import (
	"mesail.dev/packages/events"
	"mesail.dev/packages/notify"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type FanoutQueueServiceTest struct {
	behaviors *BehaviorContext
}

func (test *FanoutQueueServiceTest) queue() *FanoutQueueService {
	return &FanoutQueueService{store: NotifyTestSetup{}.FanoutStore(), timestamps: &TimestampService{}}
}

func (test *FanoutQueueServiceTest) batch(domainId string) FanoutBatch {
	return FanoutBatch{DomainId: domainId, Message: events.MessageCreated{MessageId: "msg_1", ConversationId: "cnv_1"}}
}

func (test *FanoutQueueServiceTest) remaining() []notify.Recipient {
	return []notify.Recipient{{MemberId: "member-2", Kind: "dm"}, {MemberId: "member-3", Kind: "mention"}}
}

func (test *FanoutQueueServiceTest) TestSpillStoresTheMessageAndTheRecipients() {
	domainId := NotifyTestSetup{}.Domain()
	queue := test.queue()
	err := queue.Spill(test.batch(domainId), test.remaining())
	waiting, _ := queue.Waiting(domainId, 10)
	payload, recipients, readFailure := queue.Read(waiting[0])
	Assert.IsTrue(err == nil && readFailure == nil, "spill failed")
	Assert.Equals(1, len(waiting))
	Assert.Equals("msg_1", payload.MessageId)
	Assert.Equals(test.remaining(), recipients)
}

func (test *FanoutQueueServiceTest) TestSpillWithNoRecipientsStoresNothing() {
	domainId := NotifyTestSetup{}.Domain()
	queue := test.queue()
	err := queue.Spill(test.batch(domainId), []notify.Recipient{})
	waiting, _ := queue.Waiting(domainId, 10)
	Assert.IsTrue(err == nil, "spill failed")
	Assert.Equals(0, len(waiting))
}

func (test *FanoutQueueServiceTest) TestReadRefusesABrokenRecord() {
	_, _, err := test.queue().Read(FanoutRecord{Message: "{", Recipients: []string{}})
	Assert.IsTrue(err != nil, "a broken record was read")
}

func (test *FanoutQueueServiceTest) TestSaveKeepsTheRestAndFinishesWhenEmpty() {
	domainId := NotifyTestSetup{}.Domain()
	queue := test.queue()
	queue.Spill(test.batch(domainId), test.remaining())
	waiting, _ := queue.Waiting(domainId, 10)
	partly := queue.Save(waiting[0], test.remaining()[1:])
	still, _ := queue.Waiting(domainId, 10)
	_, left, _ := queue.Read(still[0])
	err := queue.Save(waiting[0], []notify.Recipient{})
	done, _ := queue.Waiting(domainId, 10)
	Assert.IsTrue(partly == nil && err == nil, "save failed")
	Assert.Equals(test.remaining()[1:], left)
	Assert.Equals(0, len(done))
}
