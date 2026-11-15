package core

import (
	"errors"
	"testing"
)

type numbered interface {
	error
	Number() int
}

func wantCase(t *testing.T, failure numbered, number int, text string) {
	t.Helper()
	if failure.Number() != number || failure.Error() != text {
		t.Fatalf("got %d %q, want %d %q", failure.Number(), failure.Error(), number, text)
	}
}

func wantField(t *testing.T, failure *InvalidRequest, field string) {
	t.Helper()
	if failure.Field != field {
		t.Fatalf("got field %q, want %q", failure.Field, field)
	}
}

func TestInvalidRequestCasesHaveTheirNumbersAndFields(t *testing.T) {
	wantCase(t, InvalidRequest{}.LimitOutOfRange(), 1513, "limit must be between 1 and 100")
	wantField(t, InvalidRequest{}.LimitOutOfRange(), "limit")
	wantCase(t, InvalidRequest{}.BadCursor(), 1514, "the cursor is not valid")
	wantField(t, InvalidRequest{}.BadCursor(), "cursor")
	wantCase(t, InvalidRequest{}.UnknownReaction(), 1515, "the reaction is not a known reaction name")
	wantField(t, InvalidRequest{}.UnknownReaction(), "reaction")
	wantCase(t, InvalidRequest{}.EditedBodySize(), 1516, "the body must have 1 to 40000 characters")
	wantField(t, InvalidRequest{}.EditedBodySize(), "body")
	wantCase(t, InvalidRequest{}.RecipientsOnlyInDirect(), 1517, "recipients can be added only to a direct conversation")
	wantCase(t, InvalidRequest{}.UnknownAttachment("att_1", "attachmentIds[0]"), 1518, "no such attachment: att_1")
	wantField(t, InvalidRequest{}.UnknownAttachment("att_1", "attachmentIds[0]"), "attachmentIds[0]")
	wantCase(t, InvalidRequest{}.AttachmentNotReady("att_1", "attachmentIds[0]"), 1519, "the attachment is not ready: att_1")
	wantCase(t, InvalidRequest{}.DirectTooLarge(100), 1520, "a direct conversation has at most 100 member participants")
	wantField(t, InvalidRequest{}.DirectTooLarge(100), "recipients")
	wantCase(t, InvalidRequest{}.MissingSubject(), 1521, "an email needs a subject")
	wantField(t, InvalidRequest{}.MissingSubject(), "subject")
	wantCase(t, InvalidRequest{}.TooManyWrites(), 1522, "this message needs too many writes; send it to fewer new recipients")
	wantCase(t, InvalidRequest{}.BadClientMessageId(), 1523, "clientMessageId must have 1 to 200 characters")
	wantField(t, InvalidRequest{}.BadClientMessageId(), "clientMessageId")
	wantCase(t, InvalidRequest{}.BodyTooLong("body"), 1524, "the body may have at most 40000 characters")
	wantCase(t, InvalidRequest{}.EmptyMessage("body"), 1525, "the message needs a body or an attachment")
	wantCase(t, InvalidRequest{}.TooManyAttachments("attachmentIds"), 1526, "a message may have at most 10 attachments")
	wantCase(t, InvalidRequest{}.BadAttachmentId("attachmentIds"), 1527, "an attachment id must start with att_")
	wantCase(t, InvalidRequest{}.TooManyRecipients(), 1528, "a message may have at most 20 recipients")
	wantCase(t, InvalidRequest{}.IncompleteRecipient(), 1529, "each recipient needs an address, a role, a delivery and an expected kind")
	wantCase(t, InvalidRequest{}.BccChat(), 1530, "a Bcc recipient gets email only")
	wantCase(t, InvalidRequest{}.RepeatedAddress(), 1531, "each address may appear only once")
	wantCase(t, InvalidRequest{}.NoRecipients(), 1532, "a new conversation needs recipients")
	wantCase(t, InvalidRequest{}.AlsoWithoutThread(), 1533, "alsoToConversation is allowed only with threadRootId")
	wantField(t, InvalidRequest{}.AlsoWithoutThread(), "alsoToConversation")
	wantCase(t, InvalidRequest{}.ForeignFromAddress(), 1534, "the From address must be one of your own addresses")
	wantField(t, InvalidRequest{}.ForeignFromAddress(), "fromAddress")
	wantCase(t, InvalidRequest{}.ChannelEmail("dev@example.com"), 1535, "a channel gets chat only: dev@example.com")
	wantField(t, InvalidRequest{}.ChannelEmail("dev@example.com"), "recipients")
	wantCase(t, InvalidRequest{}.ChannelNotAlone(), 1536, "a channel must be the only recipient of a new message, in To")
}

func TestRefusalCasesHaveTheirNumbers(t *testing.T) {
	wantCase(t, Forbidden{}.NotTheAuthor(), 1537, "only the author can change this message")
	wantCase(t, NotFound{}.NoConversation(), 1538, "no such conversation")
	wantCase(t, NotFound{}.NoMessage(), 1539, "no such message")
	wantCase(t, NotFound{}.NoAttachment(), 1540, "no such attachment")
	wantCase(t, NotFound{}.NoInboundMessage(), 1541, "no such inbound message")
	wantCase(t, NotFound{}.NoOutboxDocument(), 1542, "no such outbox document")
	wantCase(t, NotFound{}.NoOutboxCollection(), 1543, "no such outbox collection")
	wantCase(t, NotFound{}.NoDeadEvent(), 1544, "no such dead event")
	wantCase(t, Conflict{}.MessageDeleted(), 1545, "the message is deleted")
	wantCase(t, Conflict{}.ChannelArchived(), 1546, "the channel is archived and read-only")
	wantCase(t, RecipientsChanged{}.Of([]string{"bo@example.com"}), 1547, "the recipients changed: bo@example.com")
	wantCase(t, TooLarge{}.StoredMessage(), 1548, "the message is too large to store; send a shorter body")
	wantCase(t, TooLarge{}.AttachmentTotal(), 1549, "the attachments of a message may have at most 50 MiB together")
	wantCase(t, ChatNotAllowed{}.For("carol@partner.org"), 1550, "chat is not allowed for carol@partner.org")
}

func TestUnavailableCasesHaveTheirNumbers(t *testing.T) {
	cause := errors.New("cause")
	wantCase(t, Unavailable{}.Store(cause), 1551, "the message store is not available")
	wantCase(t, Unavailable{}.Directory(cause), 1552, "the directory is not available")
	wantCase(t, Unavailable{}.AttachmentsService(cause), 1553, "the attachments service is not available")
	wantCase(t, Unavailable{}.Renderer(cause), 1554, "the text renderer is not available")
	wantCase(t, Unavailable{}.InboundStorage(cause), 1555, "the messaging storage is not available")
	wantCase(t, Unavailable{}.NotStoredYet("the channel conversation"), 1556, "the channel conversation is not stored yet")
	wantCase(t, Unavailable{}.NoServiceToken(cause), 1557, "no service token for the domain")
	wantCase(t, Unavailable{}.ServiceAnswered(502), 1558, "the service answered 502")
	wantCase(t, Unavailable{}.ServiceDown(cause), 1559, "the service is not available")
	wantCase(t, Unavailable{}.NoResolveRoute(), 1560, "the directory does not know the resolve route")
	wantCase(t, Unavailable{}.RelayAnswered(500), 1561, "the outbox relay route answered 500")
	wantCase(t, Unavailable{}.OutboxSettings(cause), 1562, "the outbox settings are missing")
}

func TestSyncCasesHaveTheirNumbers(t *testing.T) {
	wantCase(t, EditConflict{}.Of(MessageView{}), 1563, "the message changed since you read it")
	wantCase(t, InvalidRequest{}.SyncCursorCount(), 1564, "a sync takes 1 to 50 cursors")
	wantField(t, InvalidRequest{}.SyncCursorCount(), "cursors")
	wantCase(t, InvalidRequest{}.WrittenAtFormat(), 1565, "writtenAt must be a date and time like 2026-11-14T08:12:00Z")
	wantField(t, InvalidRequest{}.WrittenAtFormat(), "writtenAt")
}

func TestConflictsNameTheirCode(t *testing.T) {
	codes := Conflict{}.MessageDeleted().Code + " " + Conflict{}.ChannelArchived().Code
	if codes != "message_deleted conflict" {
		t.Fatalf("got the codes %s", codes)
	}
}

func TestAnUnavailableCaseKeepsItsCause(t *testing.T) {
	cause := errors.New("cause")
	if !errors.Is(Unavailable{}.Store(cause), cause) {
		t.Fatal("the cause is lost")
	}
}
