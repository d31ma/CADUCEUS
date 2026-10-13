import "encoding/json"

// UsersMergeTest checks the merge UsersHelper.UpsertByEmail writes against
// the patch the JavaScript repository sent for the same stored user.
// @Test(UsersTestSetup)
type UsersMergeTest struct {
	behaviors *BehaviorContext
}

func (test *UsersMergeTest) object(text string) map[string]any {
	object := map[string]any{}
	_ = json.Unmarshal([]byte(text), &object)
	return object
}

func (test *UsersMergeTest) merged() string {
	existing := UsersRecords{}.Normalized(test.object(`{"aliases":["old@example.com"],"calendarVisibility":"full","domains":["example.com","other.org"],"email":"ada@example.com","grammarCheckEnabled":true,"id":"u1","interfaceFont":"serif","interfaceLocale":"de","mailPreferences":{"workspaceLayout":"list"},"mailboxNames":{"example.com":"Work"},"mcp":{"allowedDomains":["other.org"],"enabled":true},"name":"Old","unreadBadgeDomain":"other.org","unreadBadgeScope":"domain"}`))
	given := test.object(`{"composerFont":"serif","domains":["example.com","other.org"],"email":"ADA@example.com","interfaceLocale":"fr","mailPreferences":null,"name":"New"}`)
	merge := UsersMerge{Existing: existing, Normalized: UsersRecords{}.Normalized(given), Given: given}
	return StoredRecords{}.JsonText(UsersRecords{}.Normalized(merge.Merged()))
}

func (test *UsersMergeTest) TestKeepsTheStoredChoices() {
	Assert.Equals(`{"aliases":["old@example.com"],"calendarVisibility":"full","categoryCatalog":{"example.com":{},"other.org":{}},"categoryMappings":{"example.com":{"archive":"archive","drafts":"drafts","inbox":"inbox","sent":"sent","spam":"spam","trash":"trash"},"other.org":{"archive":"archive","drafts":"drafts","inbox":"inbox","sent":"sent","spam":"spam","trash":"trash"}},"composerFont":"interface","domains":["example.com","other.org"],"email":"ada@example.com","githubProfileEnrichment":false,"grammarCheckDisabledRules":[],"grammarCheckEnabled":true,"grammarCheckLanguage":"auto","id":"u1","interfaceFont":"serif","interfaceLocale":"fr","keyboardShortcutOverrides":{},"mailPreferences":{"autoBccBySender":{},"autoCcBySender":{},"backspaceAction":"archive","conversationOrder":"ascending","defaultFrom":"","defaultReplyAction":"reply","defaultSendAction":"send","detailedHeaders":false,"detailedNames":false,"displayAttachmentPreviews":true,"emailRenderMode":"theme","findInMailDisabledDomains":[],"interfaceScale":1,"markAsReadDelay":500,"restrictMessageWidth":false,"richText":true,"showUnreadForAllFolders":false,"spellcheck":true,"spellcheckLanguage":"","swipeEnabled":true,"undoSendDelay":5000,"unsnoozeToTop":true,"use24HourClock":false,"workspaceLayout":"split"},"mailboxNames":{"example.com":"Work"},"mcp":{"accessLevel":"read-only","allowedDomains":["other.org"],"enabled":true,"excludedFolders":{}},"name":"New","newMailNotificationActions":true,"newMailNotificationDomains":["example.com","other.org"],"newMailNotificationExternalAccounts":[],"newMailNotificationPrivacy":"full","notificationSounds":{"confirm":"bell","hit-send":"pop","new-mail":"chime","send":"pulse"},"remoteImagePolicy":"block","themePreferences":{"activeTheme":"automatic","darkTheme":"dark","lightTheme":"light"},"trustedImageSenders":[],"unreadBadgeDomain":"other.org","unreadBadgeScope":"domain"}`, test.merged())
}
