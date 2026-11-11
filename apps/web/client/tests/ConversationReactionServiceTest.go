// @Test(ConversationTestSetup, {"partition":"tests"})
type ConversationReactionServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ConversationReactionServiceTest) TestReactionsAreSortedByCountThenName() {
	reactions := (&ConversationReactionService{}).Reactions(map[string]int{"party": 1, "thumbs_up": 3, "eyes": 1}, nil)
	Assert.Equals(3, len(reactions))
	Assert.Equals("thumbs_up", reactions[0].Emoji)
	Assert.Equals(3, reactions[0].Count)
	Assert.Equals("eyes", reactions[1].Emoji)
	Assert.Equals("party", reactions[2].Emoji)
}

func (test *ConversationReactionServiceTest) TestReactionsShowTheGlyph() {
	reactions := (&ConversationReactionService{}).Reactions(map[string]int{"thumbs_up": 3, "pray": 1}, nil)
	Assert.Equals("👍", reactions[0].Glyph)
	Assert.Equals("🙏", reactions[1].Glyph)
	Assert.Equals("thank you: 1", reactions[1].Label)
}

func (test *ConversationReactionServiceTest) TestMyReactionIsMarkedAndNamed() {
	reactions := (&ConversationReactionService{}).Reactions(map[string]int{"thumbs_up": 3, "party": 1}, []string{"thumbs_up"})
	Assert.IsTrue(reactions[0].Mine)
	Assert.Equals("thumbs up: 3, including you", reactions[0].Label)
	Assert.IsTrue(!reactions[1].Mine)
	Assert.Equals("party: 1", reactions[1].Label)
}

func (test *ConversationReactionServiceTest) TestUnknownNameShowsTheNameItself() {
	reactions := (&ConversationReactionService{}).Reactions(map[string]int{"wave": 2}, nil)
	Assert.Equals("wave", reactions[0].Emoji)
	Assert.Equals("wave", reactions[0].Glyph)
	Assert.Equals("wave: 2", reactions[0].Label)
}

func (test *ConversationReactionServiceTest) TestEmptyCountsAreSkipped() {
	reactions := (&ConversationReactionService{}).Reactions(map[string]int{"thumbs_up": 0, "party": -1, "eyes": 2}, nil)
	Assert.Equals(1, len(reactions))
	Assert.Equals("eyes", reactions[0].Emoji)
}

func (test *ConversationReactionServiceTest) TestNoCountsGiveNoReactions() {
	Assert.Equals(0, len((&ConversationReactionService{}).Reactions(nil, nil)))
}

func (test *ConversationReactionServiceTest) TestChoicesListTwelveNamesInOrder() {
	choices := (&ConversationReactionService{}).Choices(nil)
	Assert.Equals(12, len(choices))
	Assert.Equals("thumbs_up", choices[0].Emoji)
	Assert.Equals("👍", choices[0].Glyph)
	Assert.Equals("Add thumbs up", choices[0].Label)
	Assert.Equals("rocket", choices[11].Emoji)
	Assert.Equals("Add check mark", choices[8].Label)
}

func (test *ConversationReactionServiceTest) TestChoicesOfferToRemoveMine() {
	choices := (&ConversationReactionService{}).Choices([]string{"heart"})
	Assert.IsTrue(choices[2].Mine)
	Assert.Equals("Remove heart", choices[2].Label)
	Assert.IsTrue(!choices[0].Mine)
}
