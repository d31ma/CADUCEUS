// @Test(NotificationTestSetup, {"partition":"tests"})
type NotificationApiServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationApiServiceTest) build(domainId string) *NotificationApiService {
	return NotificationTestSetup{}.api(test.behaviors, NotificationTestSetup{}.access(test.behaviors, domainId))
}

func (test *NotificationApiServiceTest) TestPageSendsTheBearerToken() {
	page, problem := test.build("example.com").Page("")
	Assert.Equals("", problem)
	Assert.Equals(4, len(page.Items))
	Assert.Equals("page-2", page.NextCursor)
}

func (test *NotificationApiServiceTest) TestPageFailure() {
	_, problem := test.build("broken.example").Page("")
	Assert.Equals("error", problem)
}

func (test *NotificationApiServiceTest) TestMarkRead() {
	Assert.Equals("", test.build("example.com").MarkRead("n1"))
	Assert.Equals("error", test.build("example.com").MarkRead("n9"))
}

func (test *NotificationApiServiceTest) TestSettingsRoundTrip() {
	api := test.build("example.com")
	settings, problem := api.Settings()
	Assert.Equals("", problem)
	Assert.Equals(30, settings.EmailSummaryAfterMinutes)
	saved, problem := api.SaveSettings(NotificationSettings{EmailSummaryAfterMinutes: 15, MutedConversationIds: []string{}})
	Assert.Equals("", problem)
	Assert.Equals(15, saved.EmailSummaryAfterMinutes)
}

func (test *NotificationApiServiceTest) TestSettingsWaitWhileNobodyIsSignedIn() {
	api := NotificationTestSetup{}.api(test.behaviors, NotificationTestSetup{}.signedOutAccess(test.behaviors))
	_, problem := api.Settings()
	_, saveProblem := api.SaveSettings(NotificationSettings{})
	Assert.Equals("waiting", problem)
	Assert.Equals("waiting", saveProblem)
	Assert.Equals("waiting", api.MarkRead("n1"))
}
