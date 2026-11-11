// @Test(SearchTestSetup, {"partition":"tests"})
type SearchHighlightServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SearchHighlightServiceTest) TestPartsOfMarksEveryWordThatStartsWithATerm() {
	parts := (&SearchHighlightService{}).PartsOf("Invoices for March, invoice 22", "invoice MARCH")
	Assert.Equals([]SearchHitPart{
		{Text: "Invoices", Match: true},
		{Text: " ", Match: false},
		{Text: "for", Match: false},
		{Text: " ", Match: false},
		{Text: "March", Match: true},
		{Text: ", ", Match: false},
		{Text: "invoice", Match: true},
		{Text: " ", Match: false},
		{Text: "22", Match: false},
	}, parts)
}

func (test *SearchHighlightServiceTest) TestPartsOfKeepsTheCutMarkers() {
	parts := (&SearchHighlightService{}).PartsOf("...a café...", "café")
	Assert.Equals([]SearchHitPart{
		{Text: "...", Match: false},
		{Text: "a", Match: false},
		{Text: " ", Match: false},
		{Text: "café", Match: true},
		{Text: "...", Match: false},
	}, parts)
}

func (test *SearchHighlightServiceTest) TestPartsOfKeepsMarkupAsPlainText() {
	parts := (&SearchHighlightService{}).PartsOf("<b>bold</b>", "bold")
	Assert.Equals(7, len(parts))
	Assert.Equals("<", parts[0].Text)
	Assert.Equals(SearchHitPart{Text: "bold", Match: true}, parts[3])
	Assert.Equals("</", parts[4].Text)
}

func (test *SearchHighlightServiceTest) TestPartsOfAnEmptySnippetIsEmpty() {
	Assert.Equals(0, len((&SearchHighlightService{}).PartsOf("", "invoice")))
}

func (test *SearchHighlightServiceTest) TestPartsOfWithoutTermsMarksNothing() {
	Assert.Equals([]SearchHitPart{{Text: "invoice", Match: false}}, (&SearchHighlightService{}).PartsOf("invoice", " ... "))
}
