// @Test(MailboxTestSetup)
type EmailsRecordsTest struct {
	behaviors *BehaviorContext
}

func (test *EmailsRecordsTest) stored() map[string]any {
	return map[string]any{
		"id": "e1", "domain": "example.com", "subject": "Quarterly report", "sender": "Bob <bob@example.com>",
		"recipient": "alice@example.com", "labels": []any{"Work", "work"}, "trackedLinksJson": `[{"url":"x"}]`,
		"spamScore": nil, "clicks": []any{1.0},
	}
}

func (test *EmailsRecordsTest) TestNormalizesDefaultsAndTracking() {
	email := EmailsRecords{}.Normalize(test.stored())
	Assert.Equals("inbox", email["folder"])
	Assert.Equals(false, email["read"])
	Assert.Equals([]string{"work"}, email["labels"])
	Assert.Equals([]any{map[string]any{"url": "x"}}, email["trackedLinks"])
	Assert.Equals([]any{1.0}, email["clicks"])
	_, kept := email["spamScore"]
	Assert.IsTrue(!kept, "an empty spam score is removed")
	_, kept = email["trackedLinksJson"]
	Assert.IsTrue(!kept, "the JSON text is removed")
}

func (test *EmailsRecordsTest) TestLetsALegacyRecipientSeeInboxMail() {
	Assert.IsTrue(EmailsRecords{}.CanAccessEmail(test.stored(), "Alice@Example.com"))
	Assert.IsTrue(!(EmailsRecords{}).CanAccessEmail(test.stored(), "mallory@example.com"))
}

func (test *EmailsRecordsTest) TestTrustsOnlyAnExplicitOwner() {
	email := map[string]any{"ownerEmail": "carol@example.com", "recipient": "alice@example.com"}
	Assert.IsTrue(!(EmailsRecords{}).CanAccessEmail(email, "alice@example.com"))
	Assert.IsTrue(EmailsRecords{}.CanAccessEmail(email, "carol@example.com"))
}

func (test *EmailsRecordsTest) TestGivesSentMailToItsSender() {
	email := map[string]any{"folder": "sent", "sender": "alice@example.com", "to": []any{"bob@example.com"}}
	Assert.IsTrue(EmailsRecords{}.CanAccessEmail(email, "alice@example.com"))
	Assert.IsTrue(!(EmailsRecords{}).CanAccessEmail(email, "bob@example.com"))
}

// @ValueSource(["report", "from:bob", "subject:quarterly", "is:unread", "filename:budget"])
func (test *EmailsRecordsTest) TestMatchesASearchTerm(query string) {
	attachments := []map[string]any{{"filename": "Budget.xlsx"}}
	Assert.IsTrue(EmailsRecords{}.MatchesQuery(test.stored(), query, attachments), query)
}

// @ValueSource(["is:read", "to:bob", "has:attachment", "report missing"])
func (test *EmailsRecordsTest) TestRejectsAnUnmatchedTerm(query string) {
	Assert.IsTrue(!(EmailsRecords{}).MatchesQuery(test.stored(), query, nil), query)
}

func (test *EmailsRecordsTest) TestSelectsTheDomainFolderFirst() {
	records := EmailsRecords{}
	email := map[string]any{"domain": "example.com", "folder": "archive"}
	Assert.IsTrue(records.MatchesFolder(email, "inbox", map[string]string{"example.com": "archive"}))
	Assert.IsTrue(!records.MatchesFolder(email, "inbox", nil))
	Assert.IsTrue(records.MatchesFolder(email, "all", nil))
}

func (test *EmailsRecordsTest) TestListsNewestFirstWithinTheWindow() {
	stored := []FyloRecord{
		{Id: "a", Data: map[string]any{"id": "1", "domain": "example.com", "receivedAt": "2026-01-01"}},
		{Id: "b", Data: map[string]any{"id": "2", "domain": "example.com", "receivedAt": "2026-01-03"}},
		{Id: "c", Data: map[string]any{"id": "3", "domain": "other.com", "receivedAt": "2026-01-04"}},
		{Id: "d", Data: map[string]any{"id": "4", "domain": "example.com", "receivedAt": "2026-01-02"}},
	}
	limit := 1
	listed := EmailsRecords{}.Listed(stored, []string{"example.com"}, EmailListFilters{Offset: 1, Limit: &limit}, nil)
	Assert.Equals(1, len(listed))
	Assert.Equals("4", listed[0]["id"])
}
