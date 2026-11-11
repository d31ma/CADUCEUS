import "time"

// @Test(NotifyTestSetup, {"partition":"tests"})
type NotificationStoreServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationStoreServiceTest) stored(domainId string, memberId string, messageId string) NotificationRecord {
	record, err := NotifyTestSetup{}.NotificationStore().InsertIfAbsent(NotifyTestSetup{}.Notification(domainId, memberId, messageId))
	if err != nil {
		panic(err)
	}
	return record
}

func (test *NotificationStoreServiceTest) TestInsertIfAbsentKeepsOneRecordPerDedupeKey() {
	setup := NotifyTestSetup{}
	domainId := setup.Domain()
	first, err := setup.NotificationStore().InsertIfAbsent(setup.Notification(domainId, "member-1", "msg_1"))
	second, again := setup.NotificationStore().InsertIfAbsent(setup.Notification(domainId, "member-1", "msg_1"))
	listed, _ := setup.NotificationStore().ListForMember(domainId, "member-1")
	Assert.IsTrue(err == nil && again == nil, "insert failed")
	Assert.Equals(first.Id, second.Id)
	Assert.Equals(1, len(listed))
}

func (test *NotificationStoreServiceTest) TestFindLooksUpTheStoredId() {
	domainId := NotifyTestSetup{}.Domain()
	stored := test.stored(domainId, "member-1", "msg_2")
	found, exists, err := NotifyTestSetup{}.NotificationStore().Find(domainId, stored.Id)
	Assert.IsTrue(err == nil && exists, "not found")
	Assert.Equals("msg_2", found.MessageId)
}

func (test *NotificationStoreServiceTest) TestFindDoesNotSeeAnotherDomain() {
	stored := test.stored(NotifyTestSetup{}.Domain(), "member-1", "msg_3")
	_, exists, _ := NotifyTestSetup{}.NotificationStore().Find(NotifyTestSetup{}.Domain(), stored.Id)
	Assert.IsTrue(!exists, "another domain saw the notification")
}

func (test *NotificationStoreServiceTest) TestListForMemberListsOnlyThatMember() {
	domainId := NotifyTestSetup{}.Domain()
	test.stored(domainId, "member-1", "msg_4")
	test.stored(domainId, "member-2", "msg_4")
	listed, err := NotifyTestSetup{}.NotificationStore().ListForMember(domainId, "member-2")
	Assert.IsTrue(err == nil, "list failed")
	Assert.Equals(1, len(listed))
	Assert.Equals("member-2", listed[0].MemberId)
}

func (test *NotificationStoreServiceTest) TestFindByMessageFindsEveryMember() {
	domainId := NotifyTestSetup{}.Domain()
	test.stored(domainId, "member-1", "msg_5")
	test.stored(domainId, "member-2", "msg_5")
	found, err := NotifyTestSetup{}.NotificationStore().FindByMessage(domainId, "msg_5", 10)
	Assert.IsTrue(err == nil, "find failed")
	Assert.Equals(2, len(found))
}

func (test *NotificationStoreServiceTest) TestMarkReadSetsReadAtAndStopsThePush() {
	domainId := NotifyTestSetup{}.Domain()
	stored := test.stored(domainId, "member-1", "msg_6")
	store := NotifyTestSetup{}.NotificationStore()
	err := store.MarkRead(stored, "2026-11-10T22:00:00.000Z")
	found, _, _ := store.Find(domainId, stored.Id)
	Assert.IsTrue(err == nil, "mark failed")
	Assert.Equals("2026-11-10T22:00:00.000Z", found.ReadAt)
	Assert.Equals("skipped", found.PushState)
	Assert.Equals(int64(0), found.SummaryDueAt)
}

func (test *NotificationStoreServiceTest) TestMarkAllReadMarksEachRecord() {
	domainId := NotifyTestSetup{}.Domain()
	first := test.stored(domainId, "member-1", "msg_7")
	second := test.stored(domainId, "member-1", "msg_8")
	store := NotifyTestSetup{}.NotificationStore()
	err := store.MarkAllRead([]NotificationRecord{first, second}, (&TimestampService{}).Now())
	unread, _ := store.FindUnreadInConversation(domainId, "member-1", "cnv_1", 10)
	Assert.IsTrue(err == nil, "mark failed")
	Assert.Equals(0, len(unread))
}

func (test *NotificationStoreServiceTest) TestFindUnreadInConversationSkipsReadOnes() {
	domainId := NotifyTestSetup{}.Domain()
	read := test.stored(domainId, "member-1", "msg_9")
	test.stored(domainId, "member-1", "msg_10")
	store := NotifyTestSetup{}.NotificationStore()
	store.MarkRead(read, (&TimestampService{}).Now())
	unread, err := store.FindUnreadInConversation(domainId, "member-1", "cnv_1", 10)
	Assert.IsTrue(err == nil, "find failed")
	Assert.Equals(1, len(unread))
	Assert.Equals("msg_10", unread[0].MessageId)
}

func (test *NotificationStoreServiceTest) TestPatchPushChangesThePushState() {
	domainId := NotifyTestSetup{}.Domain()
	stored := test.stored(domainId, "member-1", "msg_11")
	store := NotifyTestSetup{}.NotificationStore()
	err := store.PatchPush(stored, "sent", 1)
	pending, _ := store.FindPendingPushes(domainId, 10)
	found, _, _ := store.Find(domainId, stored.Id)
	Assert.IsTrue(err == nil, "patch failed")
	Assert.Equals(0, len(pending))
	Assert.Equals(1, found.PushAttempts)
}

func (test *NotificationStoreServiceTest) TestFindPendingPushesFindsNewRecords() {
	domainId := NotifyTestSetup{}.Domain()
	test.stored(domainId, "member-1", "msg_12")
	pending, err := NotifyTestSetup{}.NotificationStore().FindPendingPushes(domainId, 10)
	Assert.IsTrue(err == nil, "find failed")
	Assert.Equals(1, len(pending))
}

func (test *NotificationStoreServiceTest) TestFindDueSummariesHonoursTheDueTime() {
	domainId := NotifyTestSetup{}.Domain()
	test.stored(domainId, "member-1", "msg_13")
	store := NotifyTestSetup{}.NotificationStore()
	due, err := store.FindDueSummaries(domainId, time.Now().Unix(), 10)
	early, _ := store.FindDueSummaries(domainId, time.Now().Unix()-3600, 10)
	Assert.IsTrue(err == nil, "find failed")
	Assert.Equals(1, len(due))
	Assert.Equals(0, len(early))
}

func (test *NotificationStoreServiceTest) TestMarkSummarizedEndsTheSummaryWait() {
	domainId := NotifyTestSetup{}.Domain()
	stored := test.stored(domainId, "member-1", "msg_14")
	store := NotifyTestSetup{}.NotificationStore()
	err := store.MarkSummarized(stored, (&TimestampService{}).Now())
	due, _ := store.FindDueSummaries(domainId, time.Now().Unix(), 10)
	Assert.IsTrue(err == nil, "mark failed")
	Assert.Equals(0, len(due))
}
