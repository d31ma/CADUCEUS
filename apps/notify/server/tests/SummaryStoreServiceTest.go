// @Test(NotifyTestSetup, {"partition":"tests"})
type SummaryStoreServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SummaryStoreServiceTest) TestInsertIfAbsentKeepsOneSummaryPerBatch() {
	setup := NotifyTestSetup{}
	record := setup.StagedSummary(setup.Domain())
	first, err := setup.SummaryStore().InsertIfAbsent(record)
	second, again := setup.SummaryStore().InsertIfAbsent(record)
	Assert.IsTrue(err == nil && again == nil, "insert failed")
	Assert.Equals(first.Id, second.Id)
	Assert.IsTrue(first.OutboxWaiting, "the summary is not waiting in the outbox")
}

func (test *SummaryStoreServiceTest) TestInsertRegistersTheOutboxCollection() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	setup.SummaryStore().InsertIfAbsent(setup.StagedSummary(domainId))
	err := setup.Registry().Check(setup.SummaryStore().Collection(domainId))
	Assert.IsTrue(err == nil, "collection not registered")
}

func (test *SummaryStoreServiceTest) TestFindReadsTheSummary() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	stored, _ := setup.SummaryStore().InsertIfAbsent(setup.StagedSummary(domainId))
	found, exists, err := setup.SummaryStore().Find(domainId, stored.Id)
	Assert.IsTrue(err == nil && exists, "not found")
	Assert.Equals("1 unread", found.Body)
}
