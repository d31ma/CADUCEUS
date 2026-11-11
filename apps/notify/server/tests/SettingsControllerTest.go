import "errors"

// @Test(NotifyTestSetup, {"partition":"tests"})
type SettingsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *SettingsControllerTest) TestGetSettingsGivesDefaults() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	settings, err := (&SettingsController{preferences: setup.Preferences()}).GetSettings(setup.Member(member, "member-1"))
	Assert.IsTrue(err == nil, "get failed")
	Assert.Equals(0, settings.EmailSummaryAfterMinutes)
}

func (test *SettingsControllerTest) TestPutSettingsRefusesTooManyMinutes() {
	setup := NotifyTestSetup{}
	member, _ := setup.Realms(setup.Domain())
	_, err := (&SettingsController{preferences: setup.Preferences()}).PutSettings(setup.Member(member, "member-1"), NotifySettings{EmailSummaryAfterMinutes: 2000})
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(err, &invalid))
}
