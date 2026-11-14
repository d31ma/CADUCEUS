package core

import "strconv"

type InvalidRequest struct {
	Message     string
	Field       string
	ErrorNumber int
}

func (InvalidRequest) LimitOutOfRange() *InvalidRequest {
	return &InvalidRequest{Message: "limit must be between 1 and 100", Field: "limit", ErrorNumber: 1513}
}

func (InvalidRequest) BadCursor() *InvalidRequest {
	return &InvalidRequest{Message: "the cursor is not valid", Field: "cursor", ErrorNumber: 1514}
}

func (InvalidRequest) UnknownReaction() *InvalidRequest {
	return &InvalidRequest{Message: "the reaction is not a known reaction name", Field: "reaction", ErrorNumber: 1515}
}

func (InvalidRequest) EditedBodySize() *InvalidRequest {
	return &InvalidRequest{Message: "the body must have 1 to 40000 characters", Field: "body", ErrorNumber: 1516}
}

func (InvalidRequest) RecipientsOnlyInDirect() *InvalidRequest {
	return &InvalidRequest{Message: "recipients can be added only to a direct conversation", Field: "recipients", ErrorNumber: 1517}
}

func (InvalidRequest) UnknownAttachment(attachmentId string, field string) *InvalidRequest {
	return &InvalidRequest{Message: "no such attachment: " + attachmentId, Field: field, ErrorNumber: 1518}
}

func (InvalidRequest) AttachmentNotReady(attachmentId string, field string) *InvalidRequest {
	return &InvalidRequest{Message: "the attachment is not ready: " + attachmentId, Field: field, ErrorNumber: 1519}
}

func (InvalidRequest) DirectTooLarge(most int) *InvalidRequest {
	return &InvalidRequest{Message: "a direct conversation has at most " + strconv.Itoa(most) + " member participants", Field: "recipients", ErrorNumber: 1520}
}

func (InvalidRequest) MissingSubject() *InvalidRequest {
	return &InvalidRequest{Message: "an email needs a subject", Field: "subject", ErrorNumber: 1521}
}

func (InvalidRequest) TooManyWrites() *InvalidRequest {
	return &InvalidRequest{Message: "this message needs too many writes; send it to fewer new recipients", Field: "recipients", ErrorNumber: 1522}
}

func (InvalidRequest) BadClientMessageId() *InvalidRequest {
	return &InvalidRequest{Message: "clientMessageId must have 1 to 200 characters", Field: "clientMessageId", ErrorNumber: 1523}
}

func (InvalidRequest) BodyTooLong(field string) *InvalidRequest {
	return &InvalidRequest{Message: "the body may have at most 40000 characters", Field: field, ErrorNumber: 1524}
}

func (InvalidRequest) EmptyMessage(field string) *InvalidRequest {
	return &InvalidRequest{Message: "the message needs a body or an attachment", Field: field, ErrorNumber: 1525}
}

func (InvalidRequest) TooManyAttachments(field string) *InvalidRequest {
	return &InvalidRequest{Message: "a message may have at most 10 attachments", Field: field, ErrorNumber: 1526}
}

func (InvalidRequest) BadAttachmentId(field string) *InvalidRequest {
	return &InvalidRequest{Message: "an attachment id must start with att_", Field: field, ErrorNumber: 1527}
}

func (InvalidRequest) TooManyRecipients() *InvalidRequest {
	return &InvalidRequest{Message: "a message may have at most 20 recipients", Field: "recipients", ErrorNumber: 1528}
}

func (InvalidRequest) IncompleteRecipient() *InvalidRequest {
	return &InvalidRequest{Message: "each recipient needs an address, a role, a delivery and an expected kind", Field: "recipients", ErrorNumber: 1529}
}

func (InvalidRequest) BccChat() *InvalidRequest {
	return &InvalidRequest{Message: "a Bcc recipient gets email only", Field: "recipients", ErrorNumber: 1530}
}

func (InvalidRequest) RepeatedAddress() *InvalidRequest {
	return &InvalidRequest{Message: "each address may appear only once", Field: "recipients", ErrorNumber: 1531}
}

func (InvalidRequest) NoRecipients() *InvalidRequest {
	return &InvalidRequest{Message: "a new conversation needs recipients", Field: "recipients", ErrorNumber: 1532}
}

func (InvalidRequest) AlsoWithoutThread() *InvalidRequest {
	return &InvalidRequest{Message: "alsoToConversation is allowed only with threadRootId", Field: "alsoToConversation", ErrorNumber: 1533}
}

func (InvalidRequest) ForeignFromAddress() *InvalidRequest {
	return &InvalidRequest{Message: "the From address must be one of your own addresses", Field: "fromAddress", ErrorNumber: 1534}
}

func (InvalidRequest) ChannelEmail(address string) *InvalidRequest {
	return &InvalidRequest{Message: "a channel gets chat only: " + address, Field: "recipients", ErrorNumber: 1535}
}

func (InvalidRequest) ChannelNotAlone() *InvalidRequest {
	return &InvalidRequest{Message: "a channel must be the only recipient of a new message, in To", Field: "recipients", ErrorNumber: 1536}
}

func (failure *InvalidRequest) Error() string {
	return failure.Message
}

func (failure *InvalidRequest) Number() int {
	return failure.ErrorNumber
}
