// @Test(NotifyTestSetup, {"partition":"tests"})
type ThreadStoreServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ThreadStoreServiceTest) TestAddParticipantKeepsEachMemberOnce() {
	domainId := NotifyTestSetup{}.Domain()
	store := NotifyTestSetup{}.ThreadStore()
	err := store.AddParticipant(domainId, "msg_root", "member-1")
	store.AddParticipant(domainId, "msg_root", "member-2")
	store.AddParticipant(domainId, "msg_root", "member-1")
	participants, _ := store.Participants(domainId, "msg_root")
	Assert.IsTrue(err == nil, "add failed")
	Assert.Equals(2, len(participants))
}

func (test *ThreadStoreServiceTest) TestParticipantsOfAnUnknownThreadIsEmpty() {
	participants, err := NotifyTestSetup{}.ThreadStore().Participants(NotifyTestSetup{}.Domain(), "msg_none")
	Assert.IsTrue(err == nil, "read failed")
	Assert.Equals(0, len(participants))
}
