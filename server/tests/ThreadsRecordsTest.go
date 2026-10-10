// @Test(MailboxTestSetup)
type ThreadsRecordsTest struct {
	behaviors *BehaviorContext
}

func (test *ThreadsRecordsTest) emails() []map[string]any {
	return []map[string]any{
		{"id": "1", "domain": "example.com", "subject": "Plans", "rfcMessageId": "<a@x>", "receivedAt": "2026-01-01", "sender": "bob@example.com", "to": []any{"alice@example.com"}},
		{"id": "2", "domain": "example.com", "subject": "Re: Plans", "inReplyTo": "<a@x>", "rfcMessageId": "<b@x>", "receivedAt": "2026-01-02", "sender": "alice@example.com", "to": []any{"bob@example.com"}},
		{"id": "3", "domain": "other.com", "subject": "Re: Plans", "inReplyTo": "<a@x>", "receivedAt": "2026-01-03", "sender": "eve@other.com"},
		{"id": "4", "domain": "example.com", "subject": "Lunch", "receivedAt": "2026-01-04", "sender": "carol@example.com", "recipient": "alice@example.com"},
	}
}

func (test *ThreadsRecordsTest) TestGroupsRepliesWithinOneTenant() {
	threads := ThreadsRecords{}.GroupMessagesIntoThreads(test.emails(), "alice@example.com")
	Assert.Equals(3, len(threads))
	Assert.Equals("legacy:hosted:example.com:lunch", threads[0].Id)
	Assert.Equals(1, threads[1].Count)
	Assert.Equals(2, threads[2].Count)
	Assert.Equals("Plans", threads[2].Subject)
	Assert.Equals("2", threads[2].LatestEmail["id"])
	Assert.Equals([]string{"bob@example.com", "alice@example.com"}, threads[2].Participants)
	Assert.Equals(2, threads[2].PersonalLevel)
}

func (test *ThreadsRecordsTest) TestKeepsThreadsWithAMessageInTheFolder() {
	records := ThreadsRecords{}
	threads := records.GroupMessagesIntoThreads(test.emails(), "alice@example.com")
	Assert.Equals(3, len(records.InFolder(threads, "inbox", nil)))
	Assert.Equals(0, len(records.InFolder(threads, "sent", nil)))
	Assert.Equals("2", records.Holding(threads, "2").LatestEmail["id"])
	Assert.IsTrue(records.Holding(threads, "9") == nil)
}

// @ValueSource(["Re: Fwd: Plans", "RE:FW: Plans", "  Plans  "])
func (test *ThreadsRecordsTest) TestStripsReplyPrefixes(subject string) {
	Assert.Equals("Plans", ThreadsRecords{}.BaseSubject(subject))
}

func (test *ThreadsRecordsTest) TestKeepsASubjectOfOnlyPrefixes() {
	Assert.Equals("Re:", ThreadsRecords{}.BaseSubject(" Re: "))
}

func (test *ThreadsRecordsTest) TestRatesHowPersonalAThreadIs() {
	level := PersonalLevel{}
	direct := []map[string]any{{"sender": "bob@example.com", "to": []any{"Alice <alice@example.com>"}}}
	group := []map[string]any{{"sender": "bob@example.com", "to": []any{"alice@example.com", "carol@example.com"}}}
	Assert.Equals(2, level.PersonalLevelForThread(direct, "alice@example.com"))
	Assert.Equals(1, level.PersonalLevelForThread(group, "alice@example.com"))
	Assert.Equals(0, level.PersonalLevelForThread(group, "dave@example.com"))
	Assert.Equals(0, level.PersonalLevelForThread(group, ""))
}

func (test *ThreadsRecordsTest) TestFallsBackToTheEnvelopeRecipient() {
	recipients := PersonalLevel{}.RecipientsFromHeaders(map[string]any{}, "Alice <alice@example.com>")
	Assert.Equals([]string{"alice@example.com"}, recipients["to"])
	Assert.Equals([]string{"bob@example.com"}, PersonalLevel{}.RecipientsFromHeaders(map[string]any{"cc": "Bob <BOB@example.com>"}, "x@y.z")["cc"])
}
