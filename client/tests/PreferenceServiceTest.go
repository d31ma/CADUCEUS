// PreferenceServiceTest covers the document-wide preferences and the other
// shell services' state that needs no server.
// @Test(ShellTestSetup, {"partition":"tests"})
type PreferenceServiceTest struct {
	behaviors *BehaviorContext
}

func (test *PreferenceServiceTest) TestStoredChoicesShapeTheToggleLabel() {
	preference := &PreferenceService{}
	preference.Start()
	preference.HearStorage(StoredValues{Session: map[string]string{"caduceus_interface_font": "serif"}, Local: map[string]string{"caduceus-theme-preferences": `{"activeTheme":"dark"}`}})
	Assert.Equals("Switch to light theme", preference.ThemeToggleLabel)
	preference.HearDevice(PreferencesChanged{Scheme: "dark", Language: "fr-FR"})
	preference.HearChoice(PreferencesChosen{ThemePreferences: &ThemePreferences{ActiveTheme: "light"}, AccentColor: "blue"})
	Assert.Equals("Switch to dark theme", preference.ThemeToggleLabel)
	preference.HearSession(SessionChanged{SignedIn: false})
}

func (test *PreferenceServiceTest) TestNoticesFollowConnectivity() {
	center := &NotificationCenterService{}
	center.Start()
	center.HearFeatures(BrowserFeatures{Notifications: true})
	center.HearConnectivity(ConnectivityChanged{Online: false})
	Assert.Equals(1, center.Count)
	Assert.Equals("offline", center.Highest.ID)
	center.HearUpdate(UpdateReady{Version: "2"})
	center.HearStream(MailStreamHealth{Healthy: true})
	center.HearStorage(StoredValues{Session: map[string]string{}, Local: map[string]string{}})
	center.HearSession(SessionChanged{SignedIn: false, Email: "a@x.com"})
	center.Toggle()
	Assert.IsTrue(center.Open)
	center.Dismiss("update-2")
	Assert.Equals(1, center.Count)
	center.HearEscape(EscapePressed{})
	Assert.Equals(false, center.Open)
	center.RunAction("manage-accounts")
	center.Close()
}

func (test *PreferenceServiceTest) TestNavigationHighlightsTheRoute() {
	navigation := &NavigationService{}
	navigation.Start()
	navigation.HearLocation(PageLocation{Path: "/compose"})
	Assert.Equals("compose", navigation.ActiveView)
	navigation.HearStorage(StoredValues{Local: map[string]string{"caduceus-interface-locale-v1": "fr"}})
	Assert.Equals("Écrire", navigation.Text("shell.compose"))
}

func (test *PreferenceServiceTest) TestExternalFolderDialogClosesWhenIdle() {
	external := &ExternalMailboxService{}
	external.Start()
	external.HearStorage(StoredValues{Local: map[string]string{"caduceus-external-folder-open": `{"a:b":true}`}})
	external.HearSession(SessionChanged{SignedIn: false})
	external.OpenDialog("create", "missing", "")
	Assert.Equals(false, external.DialogOpen)
	external.ToggleFolder("a", "b")
	Assert.Equals(0, len(external.FoldersFor("a")))
	external.HearEscape(EscapePressed{})
	external.HearBackdrop(BackdropClicked{Dialog: "other"})
	external.HearSubmit(FormSubmitted{Form: "other"})
	external.HearToastAction(ToastActionChosen{Action: "other"})
	external.CloseDialog()
	external.SubmitDialog()
	Assert.Equals("Enter a folder name of 128 characters or fewer without leading or trailing spaces.", external.DialogError)
}

func (test *PreferenceServiceTest) TestTeamAndShareStartEmpty() {
	team := &TeamService{}
	team.Start()
	team.HearSession(SessionChanged{SignedIn: false})
	team.HearTimer(TimerElapsed{Name: "other"})
	Assert.IsTrue(team.Loading)
	share := &ShareService{}
	share.Start()
	share.HearTimer(TimerElapsed{Name: "other"})
	share.HearVisibility(VisibilityChanged{Visible: false})
	Assert.IsTrue(share.Loading)
}
