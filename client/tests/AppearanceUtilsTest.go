// AppearanceUtilsTest covers themes, fonts, the mail layout and locales.
// @Test(ShellTestSetup, {"partition":"tests"})
type AppearanceUtilsTest struct {
	behaviors *BehaviorContext
}

func (test *AppearanceUtilsTest) TestUnknownThemesFallBackToTheDefaults() {
	Assert.Equals(ThemePreferences{ActiveTheme: "automatic", LightTheme: "light", DarkTheme: "dark"}, AppearanceUtils{}.Themes(ThemePreferences{ActiveTheme: "neon", LightTheme: "dark"}))
}

func (test *AppearanceUtilsTest) TestAutomaticFollowsTheDevice() {
	themes := ThemePreferences{ActiveTheme: "automatic", LightTheme: "taiga", DarkTheme: "darkside"}
	Assert.Equals("darkside", AppearanceUtils{}.Resolved(themes, true))
	Assert.Equals("taiga", AppearanceUtils{}.Resolved(themes, false))
	Assert.Equals("dark", AppearanceUtils{}.BaseMode("darkside"))
	Assert.Equals("light", AppearanceUtils{}.Opposite("dark"))
}

func (test *AppearanceUtilsTest) TestTheOldSingleThemeIsMigrated() {
	Assert.Equals("dark", AppearanceUtils{}.StoredThemes("", "dark").ActiveTheme)
	Assert.Equals("ubuntu", AppearanceUtils{}.StoredThemes(`{"activeTheme":"ubuntu"}`, "dark").ActiveTheme)
}

func (test *AppearanceUtilsTest) TestCatalogValuesOnly() {
	Assert.Equals("plex-sans", AppearanceUtils{}.InterfaceFont("url(evil)"))
	Assert.Equals("serif", AppearanceUtils{}.ComposerFont("serif"))
	Assert.Equals("teal", AppearanceUtils{}.Accent("chartreuse"))
}

func (test *AppearanceUtilsTest) TestMailPreferencesKeepCatalogValues() {
	mail := AppearanceUtils{}.Mail(`{"workspaceLayout":"list","interfaceScale":3,"showUnreadForAllFolders":true}`)
	Assert.Equals("list", mail.WorkspaceLayout)
	Assert.Equals(1.0, mail.InterfaceScale)
	Assert.IsTrue(mail.ShowUnreadForAllFolders)
	Assert.Equals("restricted", AppearanceUtils{}.MailAttributes(MailPreferences{RestrictMessageWidth: true})["data-message-width"])
	Assert.Equals("dark", AppearanceUtils{}.ThemeAttributes(ThemePreferences{ActiveTheme: "darkside"}, "darkside")["w-theme"])
	Assert.Equals("b", AppearanceUtils{}.Merged(map[string]string{"x": "a"}, map[string]string{"x": "b"})["x"])
	Assert.Equals("send", AppearanceUtils{}.ChosenMail(&MailPreferences{DefaultSendAction: "bogus"}).DefaultSendAction)
}

func (test *AppearanceUtilsTest) TestLocales() {
	Assert.Equals("Toutes les boîtes", LocaleUtils{}.Translate("fr", "shell.allInboxes", nil))
	Assert.Equals("All Inboxes", LocaleUtils{}.Translate("de", "shell.allInboxes", nil))
	Assert.Equals("missing.id", LocaleUtils{}.Translate("en", "missing.id", nil))
	Assert.Equals("fr", LocaleUtils{}.Resolve("auto", "fr-CA"))
	Assert.Equals("en", LocaleUtils{}.Resolve("auto", "ja-JP"))
	Assert.Equals("rtl", LocaleUtils{}.Direction("ar"))
	Assert.Equals("auto", LocaleUtils{}.Choice("xx"))
}
