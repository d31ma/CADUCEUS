// @Test(NotifyTestSetup, {"partition":"tests"})
type MemberSettingsStoreServiceTest struct {
	behaviors *BehaviorContext
}

func (test *MemberSettingsStoreServiceTest) TestGetGivesDefaultsForANewMember() {
	record, err := NotifyTestSetup{}.SettingsStore().Get(NotifyTestSetup{}.Domain(), "member-1")
	Assert.IsTrue(err == nil, "get failed")
	Assert.Equals(0, record.EmailSummaryAfterMinutes)
	Assert.Equals(0, len(record.MutedConversationIds))
}

func (test *MemberSettingsStoreServiceTest) TestPutPreferencesThenGet() {
	domainId := NotifyTestSetup{}.Domain()
	store := NotifyTestSetup{}.SettingsStore()
	err := store.PutPreferences(domainId, "member-1", NotifySettings{EmailSummaryAfterMinutes: 30, MutedConversationIds: []string{"cnv_1"}})
	record, _ := store.Get(domainId, "member-1")
	Assert.IsTrue(err == nil, "put failed")
	Assert.Equals(30, record.EmailSummaryAfterMinutes)
	Assert.Equals("cnv_1", record.MutedConversationIds[0])
}

func (test *MemberSettingsStoreServiceTest) TestPutProfileKeepsPreferences() {
	domainId := NotifyTestSetup{}.Domain()
	store := NotifyTestSetup{}.SettingsStore()
	store.PutPreferences(domainId, "member-1", NotifySettings{EmailSummaryAfterMinutes: 60, MutedConversationIds: []string{}})
	err := store.PutProfile(domainId, "member-1", MemberProfilePatch{DisplayName: "Ana", PrimaryAddress: "ana@example.com", State: "active"})
	record, _ := store.Get(domainId, "member-1")
	Assert.IsTrue(err == nil, "put failed")
	Assert.Equals(60, record.EmailSummaryAfterMinutes)
	Assert.Equals("ana@example.com", record.PrimaryAddress)
}

func (test *MemberSettingsStoreServiceTest) TestForMembersGivesEveryAskedMember() {
	domainId := NotifyTestSetup{}.Domain()
	store := NotifyTestSetup{}.SettingsStore()
	store.PutPreferences(domainId, "member-1", NotifySettings{EmailSummaryAfterMinutes: 15, MutedConversationIds: []string{}})
	records, err := store.ForMembers(domainId, []string{"member-1", "member-2"})
	Assert.IsTrue(err == nil, "read failed")
	Assert.Equals(15, records["member-1"].EmailSummaryAfterMinutes)
	Assert.Equals("member-2", records["member-2"].MemberId)
}
