// @Test(SearchTestSetup, {"partition":"tests"})
type SearchRowServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SearchRowServiceTest) TestRowsOfMakesOneRowPerHit() {
	rows := (&SearchRowService{highlights: &SearchHighlightService{}, times: &SearchTimeService{}}).RowsOf([]SearchHit{
		{MessageId: "m-1", ConversationId: "c-1", Snippet: "march invoice", CreatedAt: "2026-11-10T08:52:00Z"},
	}, "invoice")
	Assert.Equals([]SearchHitRow{{
		MessageId:      "m-1",
		ConversationId: "c-1",
		CreatedAt:      "2026-11-10T08:52:00Z",
		TimeLabel:      "10 Nov 2026, 08:52 UTC",
		Parts:          []SearchHitPart{{Text: "march", Match: false}, {Text: " ", Match: false}, {Text: "invoice", Match: true}},
	}}, rows)
}

func (test *SearchRowServiceTest) TestRowsOfNoHitsIsEmpty() {
	rows := (&SearchRowService{highlights: &SearchHighlightService{}, times: &SearchTimeService{}}).RowsOf(nil, "invoice")
	Assert.Equals(0, len(rows))
}
