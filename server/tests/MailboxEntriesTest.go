// @Test(MailboxTestSetup)
type MailboxEntriesTest struct {
	behaviors *BehaviorContext
}

func (test *MailboxEntriesTest) snoozed() []FyloRecord {
	return []FyloRecord{
		{Id: "d1", Data: map[string]any{"id": "s1", "ownerEmail": "Alice@example.com", "until": 30.0, "threadId": "t1", "clientRequestId": "c1"}},
		{Id: "d2", Data: map[string]any{"id": "s2", "ownerEmail": "bob@example.com", "until": 10.0, "status": "active"}},
		{Id: "d3", Data: map[string]any{"id": "s3", "until": 20.0, "status": "cancelled"}},
		{Id: "d4", Data: map[string]any{"id": "s4", "until": 90.0}},
	}
}

func (test *MailboxEntriesTest) TestSelectsActiveSnoozes() {
	records := SnoozeRecords{}
	Assert.Equals(3, len(records.Active(test.snoozed(), "")))
	Assert.Equals(2, len(records.Active(test.snoozed(), "ALICE@example.com")))
}

func (test *MailboxEntriesTest) TestListsDueSnoozesSoonestFirst() {
	due := SnoozeRecords{}.Due(SnoozeRecords{}.Active(test.snoozed(), ""), 50, 100)
	Assert.Equals(2, len(due))
	Assert.Equals("d2", due[0].Id)
	Assert.Equals(1, len(SnoozeRecords{}.Due(test.snoozed(), 50, 0)))
}

func (test *MailboxEntriesTest) TestFindsASnoozeByThreadOrRequest() {
	records := SnoozeRecords{}
	Assert.Equals("d1", records.ForThread(test.snoozed(), "t1").Id)
	Assert.Equals("d1", records.ForClientRequest(test.snoozed(), "alice@EXAMPLE.com", "c1").Id)
	Assert.Equals("", records.ForClientRequest(test.snoozed(), "bob@example.com", "c1").Id)
}

func (test *MailboxEntriesTest) TestReleasesClaimedDefaults() {
	records := []FyloRecord{
		{Id: "d1", Data: map[string]any{"id": "sg1", "defaultFor": []any{"a@x.com"}}},
		{Id: "d2", Data: map[string]any{"id": "sg2", "defaultFor": []any{"a@x.com", "b@x.com"}}},
		{Id: "d3", Data: map[string]any{"id": "sg3"}},
	}
	patches := SignatureDefaults{}.Released(records, []string{"a@x.com"}, "sg1")
	Assert.Equals(1, len(patches))
	Assert.Equals("d2", patches[0].Id)
	Assert.Equals([]any{"b@x.com"}, patches[0].Data["defaultFor"])
}

func (test *MailboxEntriesTest) TestMatchesMailActionsOfTheirOwner() {
	entries := MailActionEntries{}
	records := []FyloRecord{{Id: "D1", Data: map[string]any{"id": "m1", "ownerEmail": "Alice@example.com", "clientRequestId": "c1"}}}
	Assert.Equals("D1", entries.Matching(records, "m1", "alice@example.com").Id)
	Assert.Equals("", entries.Matching(records, "m1", "bob@example.com").Id)
	Assert.Equals(1, len(entries.OwnedBy(records, "ALICE@example.com")))
	Assert.Equals("D1", entries.ForClientRequest(records, "c1").Id)
}

func (test *MailboxEntriesTest) TestPicksTheFetchedMailAction() {
	documents := []FyloDocument{{Id: "other", Data: []byte(`{"id":"x"}`)}, {Id: "D1", Data: []byte(`{"id":"m1"}`)}}
	entry, err := MailActionEntries{}.Fetched(documents, nil, "D1")
	Assert.Equals(nil, err)
	Assert.Equals(map[string]any{"id": "m1"}, entry)
}

func (test *MailboxEntriesTest) TestStoresShareEmailIdsAsJsonText() {
	records := ThreadSharesRecords{}
	stored := records.Serialize(map[string]any{"id": "sh1", "emailIds": []any{"e1"}})
	Assert.Equals(map[string]any{"id": "sh1", "emailIdsJson": `["e1"]`}, stored)
	Assert.Equals(map[string]any{"id": "sh1", "emailIds": []any{"e1"}}, records.Deserialize(map[string]any{"id": "sh1", "emailIdsJson": `["e1", 2]`}))
	Assert.Equals([]any{}, records.Deserialize(map[string]any{"emailIdsJson": "oops"})["emailIds"])
	Assert.Equals(map[string]any{"revoked": true}, records.SerializePatch(map[string]any{"revoked": true}))
}

func (test *MailboxEntriesTest) TestFlattensARecordWithItsDocumentId() {
	record := FyloRecord{Id: "D1", Data: map[string]any{"id": "a"}}
	Assert.Equals(map[string]any{"docId": "D1", "id": "a"}, record.WithDocId())
}
