// @Test(NotifyTestSetup, {"partition":"tests"})
type FanoutStoreServiceTest struct {
	behaviors *BehaviorContext
}

func (test *FanoutStoreServiceTest) record(domainId string) FanoutRecord {
	return FanoutRecord{DomainId: domainId, MessageId: "msg_1", Message: "{}", Recipients: []string{"a", "b"}, CreatedAt: (&TimestampService{}).Now()}
}

func (test *FanoutStoreServiceTest) TestInsertIfAbsentKeepsOneRecordPerMessage() {
	domainId := NotifyTestSetup{}.Domain()
	store := NotifyTestSetup{}.FanoutStore()
	first, err := store.InsertIfAbsent(test.record(domainId))
	second, _ := store.InsertIfAbsent(test.record(domainId))
	waiting, _ := store.Waiting(domainId, 10)
	Assert.IsTrue(err == nil, "insert failed")
	Assert.Equals(first.Id, second.Id)
	Assert.Equals(1, len(waiting))
}

func (test *FanoutStoreServiceTest) TestSaveRecipientsFinishesWhenNoneAreLeft() {
	domainId := NotifyTestSetup{}.Domain()
	store := NotifyTestSetup{}.FanoutStore()
	stored, _ := store.InsertIfAbsent(test.record(domainId))
	partly := store.SaveRecipients(stored, []string{"b"})
	still, _ := store.Waiting(domainId, 10)
	err := store.SaveRecipients(stored, []string{})
	done, _ := store.Waiting(domainId, 10)
	Assert.IsTrue(partly == nil && err == nil, "save failed")
	Assert.Equals(1, len(still))
	Assert.Equals(0, len(done))
}
