import "mesail.dev/packages/events"

// @Test(NotifyTestSetup, {"partition":"tests"})
type ThreadParticipationServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ThreadParticipationServiceTest) service() *ThreadParticipationService {
	return &ThreadParticipationService{threads: NotifyTestSetup{}.ThreadStore()}
}

func (test *ThreadParticipationServiceTest) message(messageId string, authorMemberId string, threadRootId string) events.MessageCreated {
	return events.MessageCreated{MessageId: messageId, ConversationId: "cnv_1", ThreadRootId: threadRootId, AuthorKind: "member", AuthorMemberId: authorMemberId, MemberIds: []string{"member-1", "member-2", "member-3"}}
}

func (test *ThreadParticipationServiceTest) TestRecordAddsTheRootAuthor() {
	domainId := NotifyTestSetup{}.Domain()
	participants, err := test.service().Record(domainId, test.message("msg_root", "member-1", ""))
	stored, _ := NotifyTestSetup{}.ThreadStore().Participants(domainId, "msg_root")
	Assert.IsTrue(err == nil, "record failed")
	Assert.Equals(0, len(participants))
	Assert.Equals([]string{"member-1"}, stored)
}

func (test *ThreadParticipationServiceTest) TestRecordReturnsTheParticipantsBeforeTheReplyAuthor() {
	domainId := NotifyTestSetup{}.Domain()
	test.service().Record(domainId, test.message("msg_root", "member-1", ""))
	participants, err := test.service().Record(domainId, test.message("msg_reply", "member-2", "msg_root"))
	stored, _ := NotifyTestSetup{}.ThreadStore().Participants(domainId, "msg_root")
	Assert.IsTrue(err == nil, "record failed")
	Assert.Equals([]string{"member-1"}, participants)
	Assert.Equals([]string{"member-1", "member-2"}, stored)
}

func (test *ThreadParticipationServiceTest) TestRecordSkipsAnAuthorWhoIsNotAMember() {
	domainId := NotifyTestSetup{}.Domain()
	message := test.message("msg_root", "", "")
	message.AuthorKind = "external"
	_, err := test.service().Record(domainId, message)
	stored, _ := NotifyTestSetup{}.ThreadStore().Participants(domainId, "msg_root")
	Assert.IsTrue(err == nil, "record failed")
	Assert.Equals(0, len(stored))
}
