package notify

import (
	"testing"

	"mesail.dev/packages/events"
)

func channelMessage() events.MessageCreated {
	return events.MessageCreated{
		ConversationKind: "channel",
		AuthorMemberId:   "author",
		MemberIds:        []string{"author", "ana", "bo", "cy"},
	}
}

func directMessage() events.MessageCreated {
	return events.MessageCreated{
		ConversationKind: "direct",
		AuthorMemberId:   "author",
		MemberIds:        []string{"author", "ana"},
	}
}

func TestPlainChannelMessageNotifiesNobody(t *testing.T) {
	expectSame(t, RecipientRules{}.For(channelMessage(), nil, MutedSet{}), []Recipient{})
}

func TestMentionNotifiesMentionedMembersSorted(t *testing.T) {
	message := channelMessage()
	message.MentionedMemberIds = []string{"cy", "ana", "cy"}
	want := []Recipient{{MemberId: "ana", Kind: "mention"}, {MemberId: "cy", Kind: "mention"}}
	expectSame(t, RecipientRules{}.For(message, nil, MutedSet{}), want)
}

func TestMentionOfNonMemberNotifiesNobody(t *testing.T) {
	message := channelMessage()
	message.MentionedMemberIds = []string{"stranger"}
	expectSame(t, RecipientRules{}.For(message, nil, MutedSet{}), []Recipient{})
}

func TestAuthorIsNeverNotified(t *testing.T) {
	message := channelMessage()
	message.MentionedMemberIds = []string{"author"}
	message.ThreadRootId = "root"
	expectSame(t, RecipientRules{}.For(message, []string{"author"}, MutedSet{}), []Recipient{})
}

func TestDirectMessageNotifiesOtherMember(t *testing.T) {
	want := []Recipient{{MemberId: "ana", Kind: "dm"}}
	expectSame(t, RecipientRules{}.For(directMessage(), nil, MutedSet{}), want)
}

func TestDirectEmailUsesEmailKind(t *testing.T) {
	message := directMessage()
	message.IsEmail = true
	want := []Recipient{{MemberId: "ana", Kind: "email"}}
	expectSame(t, RecipientRules{}.For(message, nil, MutedSet{}), want)
}

func TestChannelEmailWithoutMentionNotifiesNobody(t *testing.T) {
	message := channelMessage()
	message.IsEmail = true
	expectSame(t, RecipientRules{}.For(message, nil, MutedSet{}), []Recipient{})
}

func TestMentionWinsOverDirect(t *testing.T) {
	message := directMessage()
	message.MentionedMemberIds = []string{"ana"}
	want := []Recipient{{MemberId: "ana", Kind: "mention"}}
	expectSame(t, RecipientRules{}.For(message, nil, MutedSet{}), want)
}

func TestDirectWinsOverThreadReply(t *testing.T) {
	message := directMessage()
	message.ThreadRootId = "root"
	want := []Recipient{{MemberId: "ana", Kind: "dm"}}
	expectSame(t, RecipientRules{}.For(message, []string{"ana"}, MutedSet{}), want)
}

func TestThreadReplyNotifiesThreadMembersWhoCanRead(t *testing.T) {
	message := channelMessage()
	message.ThreadRootId = "root"
	message.MentionedMemberIds = []string{"cy"}
	want := []Recipient{{MemberId: "bo", Kind: "thread_reply"}, {MemberId: "cy", Kind: "mention"}}
	expectSame(t, RecipientRules{}.For(message, []string{"left", "cy", "bo"}, MutedSet{}), want)
}

func TestThreadMembersIgnoredWithoutThreadRoot(t *testing.T) {
	expectSame(t, RecipientRules{}.For(channelMessage(), []string{"bo"}, MutedSet{}), []Recipient{})
}

func TestMutedConversationKeepsOnlyMentions(t *testing.T) {
	message := channelMessage()
	message.ThreadRootId = "root"
	message.MentionedMemberIds = []string{"ana"}
	muted := MutedSet{"ana": true, "bo": true}
	want := []Recipient{{MemberId: "ana", Kind: "mention"}, {MemberId: "cy", Kind: "thread_reply"}}
	expectSame(t, RecipientRules{}.For(message, []string{"bo", "cy"}, muted), want)
}

func TestMutedDirectMessageNotifiesNobody(t *testing.T) {
	expectSame(t, RecipientRules{}.For(directMessage(), nil, MutedSet{"ana": true}), []Recipient{})
}
