import (
	"encoding/json"
	"strings"
)

// ParticipantSummaryTest keeps related conversations within the search summary contract.
// @Test(ShellTestSetup, {"partition":"tests"})
type ParticipantSummaryTest struct {
	behaviors *BehaviorContext
}

func (test *ParticipantSummaryTest) TestRelatedConversationsKeepTheNewestSenderMessage() {
	participant := &ParticipantService{Address: "grace@example.net"}
	threads := participant.threads([]Email{
		{ID: "older", ThreadID: "conversation", Sender: "grace@example.net", Subject: "Plans", ReceivedAt: "2026-10-01T09:00:00Z"},
		{ID: "newer", ThreadID: "conversation", Sender: "GRACE@EXAMPLE.NET", Subject: "Re: Plans", ReceivedAt: "2026-10-02T09:00:00Z"},
		{ID: "other-sender", ThreadID: "another", Sender: "ada@example.net", Subject: "Elsewhere", ReceivedAt: "2026-10-03T09:00:00Z"},
	})
	Assert.Equals(1, len(threads))
	Assert.Equals("newer", threads[0].Id)
	Assert.Equals("Re: Plans", threads[0].Subject)
}

func (test *ParticipantSummaryTest) TestSameSubjectKeepsDistinctSummaryIdentifiers() {
	participant := &ParticipantService{Address: "grace@example.net"}
	threads := participant.threads([]Email{
		{ID: "older", Sender: "grace@example.net", Subject: "Plans", ReceivedAt: "2026-10-01T09:00:00Z"},
		{ID: "newer", Sender: "grace@example.net", Subject: "Plans", ReceivedAt: "2026-10-02T09:00:00Z"},
	})
	Assert.Equals(2, len(threads))
	Assert.Equals("newer", threads[0].Id)
	Assert.Equals("older", threads[1].Id)
}

func (test *ParticipantSummaryTest) TestSubjectFallbackGroupsOnlySummariesWithoutIdentifiers() {
	participant := &ParticipantService{Address: "grace@example.net"}
	threads := participant.threads([]Email{
		{Sender: "grace@example.net", Subject: "Plans", Snippet: "Older", ReceivedAt: "2026-10-01T09:00:00Z"},
		{Sender: "grace@example.net", Subject: "Plans", Snippet: "Newer", ReceivedAt: "2026-10-02T09:00:00Z"},
	})
	Assert.Equals(1, len(threads))
	Assert.Equals("Newer", threads[0].Snippet)
}

func (test *ParticipantSummaryTest) TestIdentifierFallbackKeepsDistinctUntitledSummaries() {
	participant := &ParticipantService{Address: "grace@example.net"}
	threads := participant.threads([]Email{
		{ID: "first", Sender: "grace@example.net"},
		{ID: "second", Sender: "grace@example.net"},
		{ID: "first", Sender: "grace@example.net"},
		{Sender: "grace@example.net"},
	})
	Assert.Equals(2, len(threads))
	Assert.Equals("first", threads[0].Id)
	Assert.Equals("second", threads[1].Id)
	Assert.Equals("(No subject)", threads[0].Subject)
}

func (test *ParticipantSummaryTest) TestRelatedSnippetsNormalizeSummaryWhitespace() {
	participant := &ParticipantService{}
	threads := participant.summaries([]Email{{ID: "message", Subject: "Earlier", Snippet: "  An\t earlier\n note  "}})
	Assert.Equals("An earlier note", threads[0].Snippet)
}

func (test *ParticipantSummaryTest) TestRelatedSnippetsLimitRunesWithoutSplittingCharacters() {
	participant := &ParticipantService{}
	threads := participant.summaries([]Email{{ID: "message", Snippet: strings.Repeat("é", 125)}})
	Assert.Equals(strings.Repeat("é", 120), threads[0].Snippet)
	Assert.Equals(120, len([]rune(threads[0].Snippet)))
}

func (test *ParticipantSummaryTest) TestRelatedSummariesDoNotExposeDetailBody() {
	participant := &ParticipantService{}
	message := Email{}
	Assert.Equals(nil, json.Unmarshal([]byte(`{"id":"message","subject":"Earlier","body":"Private detail text","messageId":"<detail@example.net>"}`), &message))
	threads := participant.summaries([]Email{message})
	Assert.Equals("", threads[0].Snippet)
	Assert.Equals("Earlier", threads[0].Subject)
	Assert.Equals("message", threads[0].Id)
}
