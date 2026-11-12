package events

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

func payloadKeys(t *testing.T, payload any) string {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	decoded := map[string]json.RawMessage{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	keys := []string{}
	for key := range decoded {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func expectKeys(t *testing.T, payload any, expected string) {
	t.Helper()
	actual := payloadKeys(t, payload)
	if actual != expected {
		t.Fatalf("keys were %q, expected %q", actual, expected)
	}
}

func TestMemberUpsertedKeys(t *testing.T) {
	expectKeys(t, MemberUpserted{}, "displayName,memberId,primaryAddress,state")
}

func TestChannelCreatedKeys(t *testing.T) {
	expectKeys(t, ChannelCreated{}, "channelId,conversationId,createdBy,emailAddress,name,topic,visibility")
}

func TestChannelMemberAddedKeys(t *testing.T) {
	expectKeys(t, ChannelMemberAdded{}, "channelId,conversationId,joinedAt,memberId")
}

func TestChannelMemberRemovedKeys(t *testing.T) {
	expectKeys(t, ChannelMemberRemoved{}, "channelId,conversationId,memberId")
}

func TestChannelArchivedKeys(t *testing.T) {
	expectKeys(t, ChannelArchived{}, "channelId,conversationId")
}

func TestMessageCreatedKeys(t *testing.T) {
	expectKeys(t, MessageCreated{}, "attachmentIds,authorAddress,authorDisplayName,authorKind,authorMemberId,body,conversationId,conversationKind,createdAt,fileNames,isEmail,memberIds,mentionedMemberIds,messageId,subject,threadRootId")
}

func TestMessageEditedKeys(t *testing.T) {
	expectKeys(t, MessageEdited{}, "body,conversationId,editedAt,memberIds,messageId")
}

func TestMessageDeletedKeys(t *testing.T) {
	expectKeys(t, MessageDeleted{}, "conversationId,deletedAt,memberIds,messageId")
}

func TestReactionChangedKeys(t *testing.T) {
	expectKeys(t, ReactionChanged{}, "added,conversationId,emoji,memberId,memberIds,messageId")
}

func TestReadPositionMovedKeys(t *testing.T) {
	expectKeys(t, ReadPositionMoved{}, "conversationId,memberId,messageId,readAt")
}

func TestEmailSendRequestedKeys(t *testing.T) {
	expectKeys(t, EmailSendRequested{}, "attachmentIds,cc,from,idempotencyKey,inReplyTo,messageId,rcpt,references,role,subject,to")
}

func TestEmailDeliveryChangedKeys(t *testing.T) {
	expectKeys(t, EmailDeliveryChanged{}, "detail,messageId,rcpt,status")
}

func TestEmailReceivedKeys(t *testing.T) {
	expectKeys(t, EmailReceived{}, "authResults,headers,inboundId,rawBlobKey,rcpts")
}

func TestEmailReceivedWithContentKeys(t *testing.T) {
	expectKeys(t, EmailReceived{Content: &EmailContent{}}, "authResults,content,headers,inboundId,rawBlobKey,rcpts")
}

func TestEmailContentKeys(t *testing.T) {
	expectKeys(t, EmailContent{Blocks: []byte("[]")}, "attachments,blocks,bodyFormat,text,truncated")
}

func TestEmailAttachmentKeys(t *testing.T) {
	expectKeys(t, EmailAttachment{ContentId: "logo"}, "blobKey,contentId,fileName,mediaType,size")
}

func TestEmailHeadersKeys(t *testing.T) {
	expectKeys(t, EmailHeaders{}, "cc,from,inReplyTo,messageId,references,subject,to")
}

func TestAuthResultsKeys(t *testing.T) {
	expectKeys(t, AuthResults{}, "dkim,dmarc,handoff,spf")
}

func TestPresenceChangedKeys(t *testing.T) {
	expectKeys(t, PresenceChanged{}, "doNotDisturbUntil,lastSeenAt,memberId,state")
}

func TestEmailReceivedRoundTrip(t *testing.T) {
	original := EmailReceived{
		InboundId:   "in_1",
		Rcpts:       []string{"ana@example.com"},
		RawBlobKey:  "example.com/in_1",
		Headers:     EmailHeaders{MessageId: "<a@b>", References: []string{"<c@d>"}, To: []string{"ana@example.com"}, Cc: []string{}},
		AuthResults: AuthResults{Spf: "pass", Dkim: "pass", Dmarc: "pass"},
	}
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	decoded := EmailReceived{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded.Headers.References[0] != "<c@d>" || decoded.AuthResults.Dmarc != "pass" {
		t.Fatalf("round trip lost data: %+v", decoded)
	}
}
