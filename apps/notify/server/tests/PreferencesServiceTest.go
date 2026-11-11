import "errors"

// @Test(NotifyTestSetup, {"partition":"tests"})
type PreferencesServiceTest struct {
	behaviors *BehaviorContext
}

func (test *PreferencesServiceTest) TestGetGivesDefaults() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	settings, err := setup.Preferences().Get(setup.Member(member, "member-1"))
	Assert.IsTrue(err == nil, "get failed")
	Assert.Equals(0, settings.EmailSummaryAfterMinutes)
	Assert.IsTrue(settings.MutedConversationIds != nil, "muted ids are null")
}

func (test *PreferencesServiceTest) TestPutStoresTheSettings() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	token := setup.Member(member, "member-1")
	_, err := setup.Preferences().Put(token, NotifySettings{EmailSummaryAfterMinutes: 45, MutedConversationIds: []string{"cnv_9"}})
	settings, _ := setup.Preferences().Get(token)
	Assert.IsTrue(err == nil, "put failed")
	Assert.Equals(45, settings.EmailSummaryAfterMinutes)
	Assert.Equals("cnv_9", settings.MutedConversationIds[0])
}

func (test *PreferencesServiceTest) TestPutRefusesABadDelay() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	_, err := setup.Preferences().Put(setup.Member(member, "member-1"), NotifySettings{EmailSummaryAfterMinutes: 5})
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid))
	Assert.Equals("emailSummaryAfterMinutes", invalid.Field)
}
