// @Test(SearchTestSetup, {"partition":"tests"})
type SearchQueryServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SearchQueryServiceTest) TestFindSendsTheBearerTokenAndAnswersRows() {
	answer := SearchTestSetup{}.Queries(test.behaviors).Find(SearchHitFilter{Query: "invoice", Limit: "20"})
	Assert.Equals("results", answer.State)
	Assert.Equals(2, len(answer.Rows))
	Assert.Equals("m-1", answer.Rows[0].MessageId)
	Assert.Equals("page-2", answer.NextCursor)
}

func (test *SearchQueryServiceTest) TestFindSendsEveryFilter() {
	answer := SearchTestSetup{}.Queries(test.behaviors).Find(SearchHitFilter{
		Query:          "filtered",
		ConversationId: "c-1",
		Author:         "bo",
		From:           "2026-11-01",
		To:             "2026-11-10",
		HasAttachment:  true,
		IsEmail:        true,
		Limit:          "20",
	})
	Assert.Equals("results", answer.State)
	Assert.Equals("m-9", answer.Rows[0].MessageId)
}

func (test *SearchQueryServiceTest) TestFindReadsTheNextPage() {
	answer := SearchTestSetup{}.Queries(test.behaviors).Find(SearchHitFilter{Query: "invoice", Cursor: "page-2", Limit: "20"})
	Assert.Equals(1, len(answer.Rows))
	Assert.Equals("", answer.NextCursor)
}

func (test *SearchQueryServiceTest) TestFindWithoutHitsIsEmpty() {
	answer := SearchTestSetup{}.Queries(test.behaviors).Find(SearchHitFilter{Query: "nothing", Limit: "20"})
	Assert.Equals("empty", answer.State)
	Assert.Equals(0, len(answer.Rows))
}

func (test *SearchQueryServiceTest) TestFindFailureKeepsTheCursorForAnotherTry() {
	answer := SearchTestSetup{}.Queries(test.behaviors).Find(SearchHitFilter{Query: "broken", Cursor: "page-5", Limit: "20"})
	Assert.Equals("error", answer.State)
	Assert.Equals("page-5", answer.NextCursor)
}

func (test *SearchQueryServiceTest) TestFindWithoutASessionIsAnError() {
	queries := SearchTestSetup{}.Queries(test.behaviors)
	queries.accounts.CurrentDomain = "example.org"
	answer := queries.Find(SearchHitFilter{Query: "invoice", Limit: "20"})
	Assert.Equals("error", answer.State)
	Assert.Equals(0, len(answer.Rows))
}

func (test *SearchQueryServiceTest) TestStateOfAnUnauthorizedAnswerEndsTheSession() {
	queries := SearchTestSetup{}.Queries(test.behaviors)
	Assert.Equals("session", queries.stateOf(&BehaviorError{Status: 401}, 0))
}

func (test *SearchQueryServiceTest) TestStateOfAnotherStatusIsAnError() {
	queries := SearchTestSetup{}.Queries(test.behaviors)
	Assert.Equals("error", queries.stateOf(&BehaviorError{Status: 503}, 0))
}
