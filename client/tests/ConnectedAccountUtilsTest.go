// ConnectedAccountUtilsTest covers the provider catalog, the OAuth redirect
// and the account card labels of the connected accounts section.
// @Test(SettingsTestSetup, {"partition":"tests"})
type ConnectedAccountUtilsTest struct {
	behaviors *BehaviorContext
}

func (test *ConnectedAccountUtilsTest) TestLaunchReadsTheProviderRedirect() {
	stored := StoredValues{Local: map[string]string{"caduceus_oauth_provider:s1": "gmail"}, Session: map[string]string{}}
	launch := ConnectedAccountUtils{}.Launch(PageLocation{Path: "/settings", Search: "?code=c1&state=s1&tab=accounts", Hash: "#accounts"}, stored)
	Assert.IsTrue(launch.HasCallback)
	Assert.Equals(false, launch.Failed)
	Assert.Equals("c1", launch.Code)
	Assert.Equals("gmail", launch.Provider)
	Assert.Equals("/settings?tab=accounts#accounts", launch.Cleaned)
}

func (test *ConnectedAccountUtilsTest) TestLaunchFallsBackToTheTabProvider() {
	stored := StoredValues{Local: map[string]string{}, Session: map[string]string{"caduceus_oauth_provider": "microsoft"}}
	launch := ConnectedAccountUtils{}.Launch(PageLocation{Path: "/settings", Search: "?code=c1&state=s2"}, stored)
	Assert.Equals("microsoft", launch.Provider)
	Assert.Equals("/settings", launch.Cleaned)
}

func (test *ConnectedAccountUtilsTest) TestLaunchMarksAProviderError() {
	launch := ConnectedAccountUtils{}.Launch(PageLocation{Path: "/settings", Search: "?error=access_denied&error_description=no"}, StoredValues{})
	Assert.IsTrue(launch.HasCallback)
	Assert.IsTrue(launch.Failed)
	Assert.Equals("", launch.Provider)
	Assert.Equals("/settings", launch.Cleaned)
}

func (test *ConnectedAccountUtilsTest) TestLaunchReadsTheOnboardingRequest() {
	Assert.IsTrue(ConnectedAccountUtils{}.Launch(PageLocation{Path: "/settings", Search: "?onboarding=accounts"}, StoredValues{}).Onboarding)
	Assert.IsTrue(ConnectedAccountUtils{}.Launch(PageLocation{Path: "/settings"}, StoredValues{Local: map[string]string{"caduceus_account_onboarding_pending": "1"}}).Onboarding)
	Assert.Equals(false, ConnectedAccountUtils{}.Launch(PageLocation{Path: "/settings"}, StoredValues{}).HasCallback)
}

// @ValueSource(["https://accounts.google.com/o/oauth2/v2/auth?x=1"])
func (test *ConnectedAccountUtilsTest) TestTrustedAcceptsTheProviderHost(address string) {
	Assert.IsTrue(ConnectedAccountUtils{}.Trusted("gmail", address))
}

// @ValueSource(["http://accounts.google.com/", "https://accounts.google.com.evil.test/", "https://login.microsoftonline.com/", "not a url %"])
func (test *ConnectedAccountUtilsTest) TestTrustedRefusesOtherHosts(address string) {
	Assert.Equals(false, ConnectedAccountUtils{}.Trusted("gmail", address))
}

func (test *ConnectedAccountUtilsTest) TestTrustedKnowsMicrosoft() {
	Assert.IsTrue(ConnectedAccountUtils{}.Trusted("microsoft", "https://login.microsoftonline.com/common/oauth2"))
}

func (test *ConnectedAccountUtilsTest) TestProvidersKeepTheSupportedEntries() {
	providers := ConnectedAccountUtils{}.Providers([]MailProvider{
		{ID: "gmail", Name: "Gmail", Provider: "gmail", Authentication: "oauth"},
		{ID: "fast", Name: "Fastmail", Provider: "imap", Authentication: "password", Guidance: "app-password"},
		{ID: "odd", Name: "Odd", Provider: "pop3", Authentication: "password"},
		{ID: "", Name: "Nameless", Provider: "imap", Authentication: "password"},
	})
	Assert.Equals(2, len(providers))
	Assert.Equals("none", providers[0].Guidance)
	Assert.Equals("app-password", providers[1].Guidance)
}

func (test *ConnectedAccountUtilsTest) TestPresetFillsTheConnectionForm() {
	settings := ConnectedAccountUtils{}.PresetSettings(MailProvider{ID: "fast"})
	Assert.Equals("manual", settings.Source)
	draft := ConnectedAccountUtils{}.Draft(settings, " Ada@Example.com ")
	Assert.Equals("ada@example.com", draft["mailboxAddress"])
	Assert.Equals("993", draft["imapPort"])
	Assert.Equals("starttls", draft["smtpSecurity"])
}

func (test *ConnectedAccountUtilsTest) TestProgressLabelCountsTheSync() {
	total := 40.0
	Assert.Equals("Syncing initial sync · 12 of 40", ConnectedAccountUtils{}.ProgressLabel(ExternalAccount{State: "syncing", Progress: &AccountProgress{Phase: "initial_sync", Completed: 12, Total: &total}}))
	Assert.Equals("Auth required", ConnectedAccountUtils{}.ProgressLabel(ExternalAccount{State: "auth_required"}))
}

func (test *ConnectedAccountUtilsTest) TestStatesAllowTheirActions() {
	Assert.IsTrue(ConnectedAccountUtils{}.CanPause("idle"))
	Assert.Equals(false, ConnectedAccountUtils{}.CanPause("paused"))
	Assert.IsTrue(ConnectedAccountUtils{}.CanReconnect("degraded"))
	Assert.IsTrue(ConnectedAccountUtils{}.OAuth("microsoft"))
	Assert.Equals(false, ConnectedAccountUtils{}.OAuth("imap"))
}

func (test *ConnectedAccountUtilsTest) TestDiagnosticsUseTheBrowserOffset() {
	diagnostics := ConnectedAccountUtils{}.Diagnostics(AccountSyncReport{Sync: AccountSync{Attempts: 3, ScheduleUpdatedAt: "2026-10-10T15:04:00Z"}}, 60)
	Assert.Equals(3, diagnostics.Attempts)
	Assert.Equals("Not scheduled", diagnostics.NextRunAt)
	Assert.Equals("Oct 10, 2026, 4:04 PM", diagnostics.UpdatedAt)
}
