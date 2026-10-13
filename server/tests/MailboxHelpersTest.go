// @Test(MailboxTestSetup)
// @Needs(EmailsHelper, AttachmentsHelper)
type MailboxHelpersTest struct {
	behaviors         *BehaviorContext
	emailsHelper      *EmailsHelper
	attachmentsHelper *AttachmentsHelper
}

func (test *MailboxHelpersTest) emails() *EmailsHelper {
	return test.emailsHelper
}

func (test *MailboxHelpersTest) TestWalksEachDomainOnceInPagesOf128() {
	walk := test.emails().IterateFolder([]string{"Example.com", "example.com", "other.com"}, "", "alice@example.com", 0)
	Assert.Equals([]string{"example.com", "other.com"}, walk.Domains)
	Assert.Equals("inbox", walk.Folder)
	Assert.Equals(128, walk.PageSize)
}

// @ValueSource([-5, 1, 512, 9000])
func (test *MailboxHelpersTest) TestBoundsTheWalkPage(size int64) {
	walk := test.emails().IterateFolder([]string{"example.com"}, "Archive", "alice@example.com", int(size))
	Assert.Equals("archive", walk.Folder)
	Assert.IsTrue(walk.PageSize >= 1 && walk.PageSize <= 512)
}

func (test *MailboxHelpersTest) TestUpdatesNoEmailForNoIds() {
	matched, err := test.emails().UpdateByIds(nil, map[string]any{"read": true})
	Assert.Equals(nil, err)
	Assert.Equals(0, len(matched))
}

func (test *MailboxHelpersTest) TestSavesNoAttachmentsWithoutStorage() {
	saved, err := test.attachmentsHelper.SaveForEmail("e1", "example.com", nil)
	Assert.Equals(nil, err)
	Assert.Equals(0, len(saved))
}

func (test *MailboxHelpersTest) TestFindsNoThreadForNoEmail() {
	thread, err := (&ThreadsHelper{}).FindForEmail([]string{"example.com"}, "", "alice@example.com")
	Assert.Equals(nil, err)
	Assert.IsTrue(thread == nil)
}

func (test *MailboxHelpersTest) TestReleasesNoDefaultsForNoAddresses() {
	Assert.Equals(nil, (&SignaturesHelper{}).ReleaseDefaults("alice@example.com", nil, "sg1"))
}

func (test *MailboxHelpersTest) TestFindsNoMailActionWithoutAnId() {
	id, entry, err := (&MailActionsHelper{}).FindById("", "alice@example.com")
	Assert.Equals("", id)
	Assert.IsTrue(entry == nil)
	Assert.Equals(nil, err)
}

func (test *MailboxHelpersTest) TestMailboxProvidersShareTheirDependencies() {
	Assert.IsTrue(test.emailsHelper != nil)
	Assert.IsTrue(test.attachmentsHelper != nil)
	Assert.IsTrue(test.emailsHelper.fyloHelper != nil)
	Assert.IsTrue(test.emailsHelper.attachmentsHelper == test.attachmentsHelper)
	Assert.IsTrue(test.emailsHelper.fyloHelper == test.attachmentsHelper.fyloHelper)
	Assert.IsTrue(test.emailsHelper.rawMessagesHelper != nil)
	Assert.IsTrue(test.emailsHelper.fyloHelper == test.emailsHelper.rawMessagesHelper.fyloHelper)
}
