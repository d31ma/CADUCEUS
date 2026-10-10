import (
	"encoding/base64"
	"net/http"
)

// InboxAttachmentRoute answers GET /inbox/{id}/attachments/{attachment_id}:
// one attachment of a message its owner may read, in base64.
type InboxAttachmentRoute struct {
	Parts MailboxParts
}

// Read answers the attachment.
func (route InboxAttachmentRoute) Read(emailId string, attachmentId string, request *http.Request) *http.Response {
	caller, refusal := route.Parts.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	if emailId == "" || attachmentId == "" {
		return Answer{}.NotFound("Attachment not found")
	}
	if accountId, stableId, external := (ExternalMessage{}).ParseId(emailId); external {
		return route.readExternal(caller, accountId, stableId, attachmentId)
	}
	_, _, refusal = route.Parts.Owned(caller, emailId)
	if refusal != nil {
		return refusal
	}
	attachment, err := route.stored(emailId, attachmentId)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if attachment == nil {
		return Answer{}.NotFound("Attachment not found")
	}
	return Answer{}.Ok(attachment)
}

func (route InboxAttachmentRoute) stored(emailId string, attachmentId string) (map[string]any, error) {
	attachment, err := route.Parts.Attachment(attachmentId)
	if err != nil || attachment == nil || attachment["emailId"] != emailId {
		return nil, err
	}
	content, err := route.Parts.AttachmentBytes(attachment)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": attachment["id"], "filename": attachment["filename"], "contentType": attachment["contentType"],
		"size": attachment["size"], "contentBase64": base64.StdEncoding.EncodeToString(content),
	}, nil
}

func (route InboxAttachmentRoute) readExternal(caller RouteCaller, accountId string, stableId string, attachmentId string) *http.Response {
	accounts := route.Parts.Provider().Accounts
	source, err := accounts.MessageSource(caller.Token, accountId, stableId)
	if err != nil {
		return accounts.Failure(err)
	}
	attachment := ExternalMessage{}.Attachment(source["rawRFC822Base64"], attachmentId)
	if attachment == nil {
		return Answer{}.NotFound("Attachment not found")
	}
	return Answer{}.Ok(attachment)
}
