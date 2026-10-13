import "net/http"

// InboxSpamRoute answers PUT /inbox/{id}/spam: the owner's spam feedback.
// Reporting moves the message to the Spam role's folder; not-spam restores
// it to the Inbox role's folder and clears its spam metadata.
type InboxSpamRoute struct {
	Parts MailboxParts
}

// Report applies the feedback and answers the message as it now reads.
func (route InboxSpamRoute) Report(id string, request *http.Request) *http.Response {
	caller, refusal := route.Parts.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	if id == "" {
		return Answer{}.NotFound("Email not found")
	}
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	spam, isFlag := body["spam"].(bool)
	if !isFlag {
		return Answer{}.BadRequest("spam must be a boolean")
	}
	documentId, email, refusal := route.Parts.Owned(caller, id)
	if refusal != nil {
		return refusal
	}
	updated, err := route.reported(caller, id, documentId, email, spam)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(updated)
}

func (route InboxSpamRoute) reported(caller RouteCaller, id string, documentId string, email map[string]any, spam bool) (map[string]any, error) {
	mappings, err := route.Parts.Mappings(caller)
	if err != nil {
		return nil, err
	}
	role := "inbox"
	patch := map[string]any{"spamScore": nil, "spamReasons": nil}
	if spam {
		role = "spam"
		patch = map[string]any{"spamReasons": []string{"user-reported"}}
	}
	patch["folder"] = TextValue{}.Or(mappings.PathFor(TextValue{}.Of(email["domain"]), role), role)
	if err := route.Parts.Update(documentId, patch); err != nil {
		return nil, err
	}
	attachments, err := route.Parts.AttachmentSummaries(id)
	if err != nil {
		return nil, err
	}
	updated := TextValue{}.Copy(email)
	for field, value := range patch {
		updated[field] = value
	}
	updated["attachments"] = attachments
	if !spam {
		delete(updated, "spamScore")
		delete(updated, "spamReasons")
	}
	return mappings.PresentEmail(updated), nil
}
