// @Test(NotifyTestSetup, {"partition":"tests"})
type DocumentsServiceTest struct {
	behaviors *BehaviorContext
}

func (test *DocumentsServiceTest) TestInsertIfAbsentWritesOnce() {
	documents := NotifyTestSetup{}.Documents()
	collection := NotifyTestSetup{}.Name("documents-test")
	first, err := documents.InsertIfAbsent(collection, map[string]any{"domainId": "a.example.com"}, DomainRecord{DomainId: "a.example.com"})
	second, again := documents.InsertIfAbsent(collection, map[string]any{"domainId": "a.example.com"}, DomainRecord{DomainId: "a.example.com"})
	Assert.IsTrue(err == nil && again == nil, "insert failed")
	Assert.IsTrue(first != "", "no id")
	Assert.Equals(first, second)
}

func (test *DocumentsServiceTest) TestFindReadsTheInsertedDocument() {
	documents := NotifyTestSetup{}.Documents()
	collection := NotifyTestSetup{}.Name("documents-test")
	id, _ := documents.InsertIfAbsent(collection, map[string]any{"domainId": "b.example.com"}, DomainRecord{DomainId: "b.example.com"})
	record := DomainRecord{}
	found, err := documents.Find(collection, id, &record)
	Assert.IsTrue(err == nil && found, "not found")
	Assert.Equals("b.example.com", record.DomainId)
}

func (test *DocumentsServiceTest) TestMatchingStopsAtTheLimit() {
	documents := NotifyTestSetup{}.Documents()
	collection := NotifyTestSetup{}.Name("documents-test")
	documents.InsertIfAbsent(collection, map[string]any{"domainId": "c.example.com"}, DomainRecord{DomainId: "c.example.com"})
	documents.InsertIfAbsent(collection, map[string]any{"domainId": "d.example.com"}, DomainRecord{DomainId: "d.example.com"})
	found := []DomainRecord{}
	err := documents.Matching(collection, map[string]any{}, 1, &found)
	Assert.IsTrue(err == nil, "matching failed")
	Assert.Equals(1, len(found))
}

func (test *DocumentsServiceTest) TestUpsertInsertsThenPatches() {
	documents := NotifyTestSetup{}.Documents()
	collection := NotifyTestSetup{}.Name("documents-test")
	where := map[string]any{"memberId": "member-1"}
	err := documents.Upsert(collection, where, MemberSettingsRecord{MemberId: "member-1", DisplayName: "Ana", MutedConversationIds: []string{}}, MemberProfilePatch{DisplayName: "Ana"})
	again := documents.Upsert(collection, where, MemberSettingsRecord{MemberId: "member-1", MutedConversationIds: []string{}}, MemberProfilePatch{DisplayName: "Ana B"})
	found := []MemberSettingsRecord{}
	documents.Matching(collection, where, 10, &found)
	Assert.IsTrue(err == nil && again == nil, "upsert failed")
	Assert.Equals(1, len(found))
	Assert.Equals("Ana B", found[0].DisplayName)
}

func (test *DocumentsServiceTest) TestPatchChangesAField() {
	documents := NotifyTestSetup{}.Documents()
	collection := NotifyTestSetup{}.Name("documents-test")
	id, _ := documents.InsertIfAbsent(collection, map[string]any{"name": "x"}, OutboxCollectionRecord{Name: "x"})
	err := documents.Patch(collection, id, OutboxCollectionRecord{Name: "y"})
	record := OutboxCollectionRecord{}
	documents.Find(collection, id, &record)
	Assert.IsTrue(err == nil, "patch failed")
	Assert.Equals("y", record.Name)
}

func (test *DocumentsServiceTest) TestDeleteRemovesTheDocument() {
	documents := NotifyTestSetup{}.Documents()
	collection := NotifyTestSetup{}.Name("documents-test")
	id, _ := documents.InsertIfAbsent(collection, map[string]any{"name": "gone"}, OutboxCollectionRecord{Name: "gone"})
	err := documents.Delete(collection, id)
	found := []OutboxCollectionRecord{}
	documents.Matching(collection, map[string]any{"name": "gone"}, 1, &found)
	Assert.IsTrue(err == nil, "delete failed")
	Assert.Equals(0, len(found))
}
