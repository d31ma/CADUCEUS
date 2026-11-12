import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"mesail.dev/packages/events"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type ChannelReadersTest struct {
	behaviors *BehaviorContext
}

func (test *ChannelReadersTest) writer() *FanoutWriterService {
	return &FanoutWriterService{store: NotifyTestSetup{}.NotificationStore(), states: &NotificationStatesService{}, timestamps: &TimestampService{}}
}

func (test *ChannelReadersTest) queue() *FanoutQueueService {
	return &FanoutQueueService{store: NotifyTestSetup{}.FanoutStore(), timestamps: &TimestampService{}}
}

func (test *ChannelReadersTest) recipients() *RecipientService {
	return &RecipientService{settings: NotifyTestSetup{}.SettingsStore()}
}

func (test *ChannelReadersTest) pushes() *PushService {
	targets := &PushTargetsService{settings: &NotifySettingsService{}, client: &http.Client{}}
	sender := &PushSenderService{subscriptions: NotifyTestSetup{}.PushStore()}
	return &PushService{store: NotifyTestSetup{}.NotificationStore(), presence: &PresenceService{settings: &NotifySettingsService{}}, targets: targets, sender: sender, states: &NotificationStatesService{}}
}

func (test *ChannelReadersTest) service() *MessageCreatedService {
	threads := &ThreadParticipationService{threads: NotifyTestSetup{}.ThreadStore()}
	return &MessageCreatedService{threads: threads, recipients: test.recipients(), writer: test.writer(), queue: test.queue(), pushes: test.pushes()}
}

func (test *ChannelReadersTest) fanout() *FanoutService {
	return &FanoutService{queue: test.queue(), recipients: test.recipients(), writer: test.writer()}
}

func (test *ChannelReadersTest) message(messageId string, authorMemberId string) events.MessageCreated {
	return events.MessageCreated{MessageId: messageId, ConversationId: "cnv_all", ConversationKind: "channel", AuthorKind: "member", AuthorMemberId: authorMemberId, AuthorDisplayName: "Ana", Body: "hello", MentionedMemberIds: []string{}, MemberIds: []string{}, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
}

func (test *ChannelReadersTest) reply(messageId string, authorMemberId string) events.MessageCreated {
	message := test.message(messageId, authorMemberId)
	message.ThreadRootId = "msg_root"
	return message
}

func (test *ChannelReadersTest) members(count int) []string {
	memberIds := []string{}
	for number := 1; number <= count; number++ {
		memberIds = append(memberIds, "member-"+strconv.Itoa(number))
	}
	return memberIds
}

func (test *ChannelReadersTest) mute(domainId string, memberId string) {
	store := NotifyTestSetup{}.SettingsStore()
	err := store.PutPreferences(domainId, memberId, NotifySettings{MutedConversationIds: []string{"cnv_all"}})
	Assert.IsTrue(err == nil, fmt.Sprintf("mute failed: %v", err))
}

func (test *ChannelReadersTest) kinds(domainId string, messageId string) map[string]string {
	records, err := NotifyTestSetup{}.NotificationStore().FindByMessage(domainId, messageId, 200)
	if err != nil {
		panic(err)
	}
	kinds := map[string]string{}
	for _, record := range records {
		kinds[record.MemberId] = record.Kind
	}
	return kinds
}

func (test *ChannelReadersTest) TestAChannelMessageWithoutReadersNotifiesTheMentionedMembers() {
	domainId := NotifyTestSetup{}.Domain()
	message := test.message("msg_1", "member-1")
	message.MentionedMemberIds = []string{"member-2", "member-3"}
	err := test.service().Apply(domainId, message)
	Assert.IsTrue(err == nil, fmt.Sprintf("apply failed: %v", err))
	Assert.Equals(map[string]string{"member-2": "mention", "member-3": "mention"}, test.kinds(domainId, "msg_1"))
}

func (test *ChannelReadersTest) TestAChannelReplyWithoutReadersNotifiesTheThreadParticipants() {
	domainId := NotifyTestSetup{}.Domain()
	test.service().Apply(domainId, test.message("msg_root", "member-1"))
	test.service().Apply(domainId, test.reply("msg_reply_1", "member-2"))
	last := test.reply("msg_reply_2", "member-3")
	last.MentionedMemberIds = []string{"member-9"}
	err := test.service().Apply(domainId, last)
	Assert.IsTrue(err == nil, fmt.Sprintf("apply failed: %v", err))
	Assert.Equals(map[string]string{"member-1": "thread_reply"}, test.kinds(domainId, "msg_reply_1"))
	Assert.Equals(map[string]string{"member-1": "thread_reply", "member-2": "thread_reply", "member-9": "mention"}, test.kinds(domainId, "msg_reply_2"))
}

func (test *ChannelReadersTest) TestAMutedThreadParticipantKeepsOnlyMentions() {
	domainId := NotifyTestSetup{}.Domain()
	test.mute(domainId, "member-1")
	test.mute(domainId, "member-2")
	test.service().Apply(domainId, test.message("msg_root", "member-1"))
	test.service().Apply(domainId, test.reply("msg_reply_1", "member-2"))
	last := test.reply("msg_reply_2", "member-3")
	last.MentionedMemberIds = []string{"member-2"}
	err := test.service().Apply(domainId, last)
	Assert.IsTrue(err == nil, fmt.Sprintf("apply failed: %v", err))
	Assert.Equals(map[string]string{"member-2": "mention"}, test.kinds(domainId, "msg_reply_2"))
}

func (test *ChannelReadersTest) TestAHundredMentionsAreAllNotifiedThroughASmallSpilledRecord() {
	domainId := NotifyTestSetup{}.Domain()
	message := test.message("msg_1", "member-0")
	message.MentionedMemberIds = test.members(100)
	err := test.service().Apply(domainId, message)
	waiting, waitingError := test.queue().Waiting(domainId, 10)
	Assert.IsTrue(err == nil && waitingError == nil, fmt.Sprintf("apply failed: %v %v", err, waitingError))
	Assert.Equals(1, len(waiting))
	encoded, _ := json.Marshal(waiting[0])
	Assert.IsTrue(len(encoded) < 16*1024, fmt.Sprintf("the spilled record has %d bytes", len(encoded)))
	progress, processFailure := test.fanout().Process(domainId, 100)
	Assert.IsTrue(processFailure == nil, fmt.Sprintf("process failed: %v", processFailure))
	Assert.Equals(false, progress.More)
	Assert.Equals(100, len(test.kinds(domainId, "msg_1")))
}
