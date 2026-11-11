import (
	"net/http"
	"strconv"
	"time"

	"mesail.dev/packages/events"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type MessageCreatedServiceTest struct {
	behaviors *BehaviorContext
}

func (test *MessageCreatedServiceTest) writer() *FanoutWriterService {
	return &FanoutWriterService{store: NotifyTestSetup{}.NotificationStore(), states: &NotificationStatesService{}, timestamps: &TimestampService{}}
}

func (test *MessageCreatedServiceTest) queue() *FanoutQueueService {
	return &FanoutQueueService{store: NotifyTestSetup{}.FanoutStore(), timestamps: &TimestampService{}}
}

func (test *MessageCreatedServiceTest) recipients() *RecipientService {
	return &RecipientService{settings: NotifyTestSetup{}.SettingsStore()}
}

func (test *MessageCreatedServiceTest) pushes() *PushService {
	targets := &PushTargetsService{settings: &NotifySettingsService{}, client: &http.Client{}}
	sender := &PushSenderService{subscriptions: NotifyTestSetup{}.PushStore()}
	return &PushService{store: NotifyTestSetup{}.NotificationStore(), presence: &PresenceService{settings: &NotifySettingsService{}}, targets: targets, sender: sender, states: &NotificationStatesService{}}
}

func (test *MessageCreatedServiceTest) service() *MessageCreatedService {
	threads := &ThreadParticipationService{threads: NotifyTestSetup{}.ThreadStore()}
	return &MessageCreatedService{threads: threads, recipients: test.recipients(), writer: test.writer(), queue: test.queue(), pushes: test.pushes()}
}

func (test *MessageCreatedServiceTest) fanout() *FanoutService {
	return &FanoutService{queue: test.queue(), recipients: test.recipients(), writer: test.writer()}
}

func (test *MessageCreatedServiceTest) members(count int) []string {
	memberIds := []string{}
	for number := 1; number <= count; number++ {
		memberIds = append(memberIds, "member-"+strconv.Itoa(number))
	}
	return memberIds
}

func (test *MessageCreatedServiceTest) message(messageId string, authorMemberId string, count int) events.MessageCreated {
	return events.MessageCreated{MessageId: messageId, ConversationId: "cnv_1", ConversationKind: "channel", AuthorKind: "member", AuthorMemberId: authorMemberId, AuthorDisplayName: "Ana", MentionedMemberIds: []string{}, MemberIds: test.members(count), CreatedAt: time.Now().UTC().Format(time.RFC3339)}
}

func (test *MessageCreatedServiceTest) reply(messageId string, authorMemberId string) events.MessageCreated {
	message := test.message(messageId, authorMemberId, 4)
	message.ThreadRootId = "msg_root"
	return message
}

func (test *MessageCreatedServiceTest) mute(domainId string, memberId string) {
	store := NotifyTestSetup{}.SettingsStore()
	if err := store.PutPreferences(domainId, memberId, NotifySettings{MutedConversationIds: []string{"cnv_1"}}); err != nil {
		panic(err)
	}
}

func (test *MessageCreatedServiceTest) kinds(domainId string, messageId string) map[string]string {
	records, err := NotifyTestSetup{}.NotificationStore().FindByMessage(domainId, messageId, 100)
	if err != nil {
		panic(err)
	}
	kinds := map[string]string{}
	for _, record := range records {
		kinds[record.MemberId] = record.Kind
	}
	return kinds
}

func (test *MessageCreatedServiceTest) TestApplyNotifiesAMentionedMemberAndNeverTheAuthor() {
	domainId := NotifyTestSetup{}.Domain()
	message := test.message("msg_1", "member-1", 3)
	message.MentionedMemberIds = []string{"member-1", "member-2"}
	err := test.service().Apply(domainId, message)
	Assert.IsTrue(err == nil, "apply failed")
	Assert.Equals(map[string]string{"member-2": "mention"}, test.kinds(domainId, "msg_1"))
}

func (test *MessageCreatedServiceTest) TestApplyTwiceCreatesNothingNew() {
	domainId := NotifyTestSetup{}.Domain()
	message := test.message("msg_1", "member-1", 3)
	message.ConversationKind = "direct"
	first := test.service().Apply(domainId, message)
	second := test.service().Apply(domainId, message)
	listed, _ := NotifyTestSetup{}.NotificationStore().ListForMember(domainId, "member-2")
	Assert.IsTrue(first == nil && second == nil, "apply failed")
	Assert.Equals(map[string]string{"member-2": "dm", "member-3": "dm"}, test.kinds(domainId, "msg_1"))
	Assert.Equals(1, len(listed))
}

func (test *MessageCreatedServiceTest) TestApplyKeepsOnlyMentionsInAMutedConversation() {
	domainId := NotifyTestSetup{}.Domain()
	test.mute(domainId, "member-2")
	test.mute(domainId, "member-3")
	message := test.message("msg_1", "member-1", 4)
	message.ConversationKind = "direct"
	message.MentionedMemberIds = []string{"member-2"}
	err := test.service().Apply(domainId, message)
	Assert.IsTrue(err == nil, "apply failed")
	Assert.Equals(map[string]string{"member-2": "mention", "member-4": "dm"}, test.kinds(domainId, "msg_1"))
}

func (test *MessageCreatedServiceTest) TestApplyNotifiesEarlierThreadParticipants() {
	domainId := NotifyTestSetup{}.Domain()
	test.service().Apply(domainId, test.message("msg_root", "member-1", 4))
	test.service().Apply(domainId, test.reply("msg_reply_1", "member-2"))
	err := test.service().Apply(domainId, test.reply("msg_reply_2", "member-3"))
	Assert.IsTrue(err == nil, "apply failed")
	Assert.Equals(map[string]string{}, test.kinds(domainId, "msg_root"))
	Assert.Equals(map[string]string{"member-1": "thread_reply"}, test.kinds(domainId, "msg_reply_1"))
	Assert.Equals(map[string]string{"member-1": "thread_reply", "member-2": "thread_reply"}, test.kinds(domainId, "msg_reply_2"))
}

func (test *MessageCreatedServiceTest) TestApplyKeepsEveryNotificationInsideTheDomain() {
	domainId := NotifyTestSetup{}.Domain()
	otherDomainId := NotifyTestSetup{}.Domain()
	test.service().Apply(otherDomainId, test.message("msg_root", "member-1", 4))
	err := test.service().Apply(domainId, test.reply("msg_reply", "member-2"))
	listed, _ := NotifyTestSetup{}.NotificationStore().ListForMember(otherDomainId, "member-1")
	Assert.IsTrue(err == nil, "apply failed")
	Assert.Equals(map[string]string{}, test.kinds(domainId, "msg_reply"))
	Assert.Equals(map[string]string{}, test.kinds(otherDomainId, "msg_reply"))
	Assert.Equals(0, len(listed))
}

func (test *MessageCreatedServiceTest) TestApplySpillsRecipientsBeyondTheBudgetAndProcessFinishesThem() {
	domainId := NotifyTestSetup{}.Domain()
	message := test.message("msg_1", "member-1", 25)
	message.ConversationKind = "direct"
	err := test.service().Apply(domainId, message)
	written := len(test.kinds(domainId, "msg_1"))
	waiting, _ := test.queue().Waiting(domainId, 10)
	progress, processFailure := test.fanout().Process(domainId, 25)
	Assert.IsTrue(err == nil && processFailure == nil, "apply failed")
	Assert.Equals(20, written)
	Assert.Equals(1, len(waiting))
	Assert.Equals(4, len(waiting[0].Recipients))
	Assert.Equals(JobProgress{Written: 5, Done: 1, More: false}, progress)
	Assert.Equals(24, len(test.kinds(domainId, "msg_1")))
}
