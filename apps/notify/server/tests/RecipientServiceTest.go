import (
	"mesail.dev/packages/events"
	"mesail.dev/packages/notify"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type RecipientServiceTest struct {
	behaviors *BehaviorContext
}

func (test *RecipientServiceTest) service() *RecipientService {
	return &RecipientService{settings: NotifyTestSetup{}.SettingsStore()}
}

func (test *RecipientServiceTest) message() events.MessageCreated {
	return events.MessageCreated{MessageId: "msg_1", ConversationId: "cnv_1", ConversationKind: "direct", AuthorKind: "member", AuthorMemberId: "member-1", MemberIds: []string{"member-1", "member-2", "member-3"}}
}

func (test *RecipientServiceTest) TestForLeavesOutAMemberWhoMutedTheConversation() {
	domainId := NotifyTestSetup{}.Domain()
	muted := NotifyTestSetup{}.SettingsStore().PutPreferences(domainId, "member-2", NotifySettings{MutedConversationIds: []string{"cnv_1"}})
	batch, err := test.service().For(domainId, test.message(), []string{})
	Assert.IsTrue(muted == nil && err == nil, "recipients failed")
	Assert.Equals([]notify.Recipient{{MemberId: "member-3", Kind: "dm"}}, batch.Recipients)
	Assert.Equals(domainId, batch.DomainId)
}

func (test *RecipientServiceTest) TestResumeKeepsTheGivenRecipientsAndLoadsSettings() {
	domainId := NotifyTestSetup{}.Domain()
	saved := NotifyTestSetup{}.SettingsStore().PutPreferences(domainId, "member-2", NotifySettings{EmailSummaryAfterMinutes: 15, MutedConversationIds: []string{}})
	recipients := []notify.Recipient{{MemberId: "member-2", Kind: "mention"}}
	batch, err := test.service().Resume(domainId, test.message(), recipients)
	Assert.IsTrue(saved == nil && err == nil, "resume failed")
	Assert.Equals(recipients, batch.Recipients)
	Assert.Equals(15, batch.Settings["member-2"].EmailSummaryAfterMinutes)
}
