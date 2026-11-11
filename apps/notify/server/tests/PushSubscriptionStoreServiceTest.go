// @Test(NotifyTestSetup, {"partition":"tests"})
type PushSubscriptionStoreServiceTest struct {
	behaviors *BehaviorContext
}

func (test *PushSubscriptionStoreServiceTest) record(domainId string, memberId string, endpoint string) PushSubscriptionRecord {
	return PushSubscriptionRecord{DomainId: domainId, MemberId: memberId, Platform: "web", Endpoint: endpoint, P256dh: "key", Auth: "auth", CreatedAt: (&TimestampService{}).Now()}
}

func (test *PushSubscriptionStoreServiceTest) TestUpsertKeepsOneRecordPerEndpoint() {
	domainId := NotifyTestSetup{}.Domain()
	store := NotifyTestSetup{}.PushStore()
	err := store.Upsert(test.record(domainId, "member-1", "https://fcm.googleapis.com/fcm/send/a"))
	again := store.Upsert(test.record(domainId, "member-2", "https://fcm.googleapis.com/fcm/send/a"))
	first, _ := store.ListForMember(domainId, "member-1")
	second, _ := store.ListForMember(domainId, "member-2")
	Assert.IsTrue(err == nil && again == nil, "upsert failed")
	Assert.Equals(0, len(first))
	Assert.Equals(1, len(second))
}

func (test *PushSubscriptionStoreServiceTest) TestDeleteForMemberLeavesOtherMembers() {
	domainId := NotifyTestSetup{}.Domain()
	store := NotifyTestSetup{}.PushStore()
	store.Upsert(test.record(domainId, "member-1", "https://fcm.googleapis.com/fcm/send/b"))
	err := store.DeleteForMember(domainId, "member-2", "https://fcm.googleapis.com/fcm/send/b")
	kept, _ := store.ListForMember(domainId, "member-1")
	removed := store.DeleteForMember(domainId, "member-1", "https://fcm.googleapis.com/fcm/send/b")
	left, _ := store.ListForMember(domainId, "member-1")
	Assert.IsTrue(err == nil && removed == nil, "delete failed")
	Assert.Equals(1, len(kept))
	Assert.Equals(0, len(left))
}

func (test *PushSubscriptionStoreServiceTest) TestDeleteRemovesById() {
	domainId := NotifyTestSetup{}.Domain()
	store := NotifyTestSetup{}.PushStore()
	store.Upsert(test.record(domainId, "member-1", "https://fcm.googleapis.com/fcm/send/c"))
	listed, _ := store.ListForMember(domainId, "member-1")
	err := store.Delete(domainId, listed[0].Id)
	left, _ := store.ListForMember(domainId, "member-1")
	Assert.IsTrue(err == nil, "delete failed")
	Assert.Equals(0, len(left))
}
