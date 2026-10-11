// ShellServiceTest covers the frame state that needs no server.
// @Test(ShellTestSetup, {"partition":"tests"})
type ShellServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ShellServiceTest) shell() *ShellService {
	shell := &ShellService{}
	shell.Start()
	return shell
}

func (test *ShellServiceTest) TestMenusAndDrawerExcludeEachOther() {
	shell := test.shell()
	shell.ToggleUserMenu()
	Assert.IsTrue(shell.UserMenuOpen)
	shell.ToggleMobileSidebar()
	Assert.Equals(false, shell.UserMenuOpen)
	Assert.IsTrue(shell.MobileSidebarOpen)
	shell.CloseMobileSidebar()
	shell.ToggleUserMenu()
	shell.CloseUserMenu()
	Assert.Equals(false, shell.UserMenuOpen || shell.MobileSidebarOpen)
}

func (test *ShellServiceTest) TestLocationSelectsTheFolderAndMailbox() {
	shell := test.shell()
	shell.HearLocation(PageLocation{Path: "/folder/sent", Search: "?domain=Example.com&account=ext_1"})
	Assert.Equals("sent", shell.ActiveFolder)
	Assert.Equals("example.com", shell.SelectedDomain)
	Assert.Equals("ext_1", shell.SelectedAccount)
	Assert.Equals(false, shell.IsRootPath)
}

func (test *ShellServiceTest) TestSidebarDisclosuresRememberTheirState() {
	shell := test.shell()
	Assert.IsTrue(shell.SectionExpanded("favorites"))
	shell.ToggleSection("favorites")
	Assert.Equals("sidebar-section collapsed", shell.SectionClass("favorites"))
	Assert.Equals(false, shell.FavoriteExpanded("sent"))
	shell.ToggleFavorite("sent")
	Assert.Equals("favorite-group", shell.FavoriteGroupClass("sent"))
}

func (test *ShellServiceTest) TestToastShowsAndItsActionRunsOnce() {
	shell := test.shell()
	shell.HearToast(ToastRequested{Message: "Saved.", ActionLabel: "Undo", Action: "undo-mail-action", Subject: "act_1"})
	Assert.IsTrue(shell.ToastVisible)
	Assert.Equals("Undo", shell.ToastActionLabel)
	shell.RunToastAction()
	Assert.Equals(false, shell.ToastVisible)
	shell.HearTimer(TimerElapsed{Name: "toast-1"})
	Assert.Equals("Saved.", shell.ToastMessage)
}

func (test *ShellServiceTest) TestScreenshotModeAndDialogs() {
	shell := test.shell()
	shell.HearSession(SessionChanged{SignedIn: false, Email: "a@x.com"})
	shell.ToggleScreenshotMode()
	Assert.IsTrue(shell.ScreenshotMode)
	shell.HearShortcut(ShortcutPressed{Command: "core:show-shortcuts"})
	Assert.IsTrue(shell.ShortcutHelpOpen)
	shell.HearEscape(EscapePressed{})
	Assert.Equals(false, shell.ShortcutHelpOpen)
	shell.HearShortcutGroups(ShortcutGroupsListed{Groups: []ShortcutGroup{{Label: "General"}}})
	Assert.Equals("General", shell.ShortcutGroups[0].Label)
	shell.HearBackdrop(BackdropClicked{Dialog: "shortcut-help"})
	shell.HearPageClick(PageClicked{InsideUserMenu: false})
	shell.OpenShortcutSettings()
	shell.CloseShortcutHelp()
	shell.HearPreferences(PreferencesLoaded{InterfaceLocale: "fr"})
	Assert.Equals("Favoris", shell.Text("shell.favorites"))
	Assert.Equals("a%20b%2Fc", shell.Encode("a b/c"))
	shell.SignOut()
}

func (test *ShellServiceTest) TestStorageRestoresScreenshotMode() {
	shell := test.shell()
	shell.HearSession(SessionChanged{SignedIn: false, Email: "a@x.com"})
	shell.HearStorage(StoredValues{Session: map[string]string{"caduceus-screenshot-mode": "on"}, Local: map[string]string{}})
	Assert.IsTrue(shell.ScreenshotMode)
	shell.HearToastAction(ToastActionChosen{Action: "nothing"})
	shell.HearVisibility(VisibilityChanged{Visible: false})
}
