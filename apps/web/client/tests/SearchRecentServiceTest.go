// @Test(SearchTestSetup, {"partition":"tests"})
type SearchRecentServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SearchRecentServiceTest) TestWithPutsTheNewestFirst() {
	Assert.Equals([]string{"march", "invoice"}, (&SearchRecentService{}).With([]string{"invoice"}, "  march "))
}

func (test *SearchRecentServiceTest) TestWithMovesARepeatedSearchToTheTop() {
	Assert.Equals([]string{"Invoice", "march"}, (&SearchRecentService{}).With([]string{"march", "invoice"}, "Invoice"))
}

func (test *SearchRecentServiceTest) TestWithKeepsFiveSearches() {
	recent := (&SearchRecentService{}).With([]string{"a", "b", "c", "d", "e"}, "f")
	Assert.Equals([]string{"f", "a", "b", "c", "d"}, recent)
}

func (test *SearchRecentServiceTest) TestWithIgnoresEmptyText() {
	Assert.Equals([]string{"a"}, (&SearchRecentService{}).With([]string{"a"}, "   "))
}

func (test *SearchRecentServiceTest) TestWithStartsFromNothing() {
	Assert.Equals([]string{"a"}, (&SearchRecentService{}).With(nil, "a"))
}
