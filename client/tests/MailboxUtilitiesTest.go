import "time"

// MailboxUtilitiesTest covers the pure utilities of the mailbox list.
// @Test(MailboxListTestSetup, {"partition":"tests"})
type MailboxUtilitiesTest struct {
	behaviors *BehaviorContext
}

func (test *MailboxUtilitiesTest) TestCurrentFindsTheTokenUnderTheCaret() {
	current := ThreadSearchUtils{}.Current("is:unread from:gra", 18)
	Assert.Equals("from", current.Token)
	Assert.Equals("gra", current.Term)
	Assert.Equals(10, current.Index)
	Assert.Equals(-1, ThreadSearchUtils{}.Current("plain words", 3).Index)
}

func (test *MailboxUtilitiesTest) TestStaticSuggestionsOfferFieldsThenValues() {
	Assert.Equals(8, len(ThreadSearchUtils{}.StaticSuggestions("", 0)))
	Assert.Equals("from", ThreadSearchUtils{}.StaticSuggestions("fr", 2)[0].Token)
	Assert.Equals("read", ThreadSearchUtils{}.StaticSuggestions("is:re", 5)[0].Term)
	Assert.Equals(`"2 days ago"`, ThreadSearchUtils{}.StaticSuggestions("since:2", 7)[0].Term)
	Assert.Equals(0, len(ThreadSearchUtils{}.StaticSuggestions("hello", 5)))
}

func (test *MailboxUtilitiesTest) TestApplyReplacesTheTokenAndPads() {
	applied := ThreadSearchUtils{}.Apply("fr", 2, SearchSuggestion{Token: "from"})
	Assert.Equals("from: ", applied.Query)
	Assert.Equals(false, applied.Submit)
	chosen := ThreadSearchUtils{}.Apply("from:grace", 10, SearchSuggestion{Token: "from", Term: ThreadSearchUtils{}.Wrap("grace@example.net"), Submit: true})
	Assert.Equals(`from:"grace@example.net" `, chosen.Query)
	Assert.IsTrue(chosen.Submit)
}

func (test *MailboxUtilitiesTest) TestSegmentsMarkTheFieldNames() {
	segments := ThreadSearchUtils{}.Segments("from:ada hello")
	Assert.Equals(2, len(segments))
	Assert.Equals("from:", segments[0].Text)
	Assert.IsTrue(segments[0].Token)
	Assert.Equals("ada hello", segments[1].Text)
}

func (test *MailboxUtilitiesTest) TestPickerPathsFollowTheServerGrammar() {
	Assert.Equals("projects/launch", CategoryPickerUtils{}.NormalizePath(" Projects/Launch "))
	Assert.Equals("", CategoryPickerUtils{}.NormalizePath("a//b"))
	Assert.Equals("", CategoryPickerUtils{}.NormalizePath("a/../b"))
}

func (test *MailboxUtilitiesTest) TestPickerItemsCountTheLabels() {
	account := CategoryAccount{Domain: "example.com", Capabilities: CategoryCapabilities{Labels: true}, Categories: []Category{{Path: "work", Kind: "label"}, {Path: "home", Kind: "label"}, {Path: "archive", Kind: "folder"}}}
	messages := []PickerMessage{{ID: "a", Domain: "example.com", Labels: []string{"work"}}, {ID: "b", Domain: "Example.com"}}
	items := CategoryPickerUtils{}.Items(account, "label", "", messages)
	Assert.Equals(2, len(items))
	Assert.Equals("mixed", items[0].SelectedState)
	Assert.Equals("none", items[1].SelectedState)
	Assert.Equals(0, len(CategoryPickerUtils{}.Items(account, "folder", "", messages)))
	Assert.Equals("example.com", CategoryPickerUtils{}.SingleDomain(messages))
}

func (test *MailboxUtilitiesTest) TestLabelPatchRemovesALabelEveryMessageHas() {
	messages := []PickerMessage{{ID: "a", Labels: []string{"work"}}}
	patch := CategoryPickerUtils{}.LabelPatch(messages, "work").(map[string]any)["labels"].(map[string]any)
	Assert.Equals(0, len(patch["add"].([]string)))
	Assert.Equals("work", patch["remove"].([]string)[0])
}

func (test *MailboxUtilitiesTest) TestDateLabelsMatchTheEnglishLocale() {
	local := time.Date(2026, 10, 11, 13, 5, 0, 0, time.Local)
	Assert.Equals("10/11/2026", DateLabelUtils{}.Numeric(local.Format(time.RFC3339)))
	Assert.Equals("", DateLabelUtils{}.Numeric("not a date"))
	Assert.Equals("Jan 5", DateLabelUtils{}.MonthDay("2026-01-05"))
	Assert.Equals("1:05 PM", DateLabelUtils{}.Clock(local, false))
	Assert.Equals("09", DateLabelUtils{}.Clock(time.Date(2026, 10, 11, 9, 0, 0, 0, time.Local), true))
	Assert.Equals("Oct 11", DateLabelUtils{}.MonthDayOf(local))
}

func (test *MailboxUtilitiesTest) TestSnoozeBadgeShowsTheDayWhenLater() {
	now := time.Date(2026, 10, 11, 8, 0, 0, 0, time.Local)
	soon := time.Date(2026, 10, 11, 9, 0, 0, 0, time.Local).Format(time.RFC3339)
	later := time.Date(2026, 10, 14, 9, 30, 0, 0, time.Local).Format(time.RFC3339)
	Assert.Equals("Snoozed until 9 AM", SnoozePresetsUtils{}.UntilLabel(soon, now, false))
	Assert.Equals("Snoozed until Oct 14, 9:30 AM", SnoozePresetsUtils{}.UntilLabel(later, now, false))
	Assert.Equals("Snoozed", SnoozePresetsUtils{}.UntilLabel("", now, false))
}

func (test *MailboxUtilitiesTest) TestComposeHandsOverThePrefill() {
	stored := ComposeUtils{}.Stored(ComposeFields{DraftID: "d1"})
	Assert.Equals(`{"draftId":"d1"}`, stored.Values["caduceus_compose_prefill"])
	Assert.Equals("caduceus_compose_prefill", ComposeUtils{}.Cleared().Removed[0])
	Assert.Equals("/compose", ComposeUtils{}.Opened().Target)
	Assert.Equals("/email/a%2Fb", ComposeUtils{}.Message("a/b").Target)
}
