import (
	"errors"
	"net/http"
)

// InboxEmailRoute answers /inbox/{id}: one hosted or connected-account
// message, read, changed or deleted by its owner.
type InboxEmailRoute struct {
	Parts MailboxParts
}

// Read answers the message with its attachment summaries and, when it came
// with a meeting invite, the linked calendar event.
func (route InboxEmailRoute) Read(id string, request *http.Request) *http.Response {
	return route.dispatch(id, request, route.readExternal, func(caller RouteCaller, _ string, email map[string]any) *http.Response {
		detail, err := route.detail(caller, id, email)
		if err != nil {
			return RouteAccess{}.Failure(err)
		}
		return Answer{}.Ok(detail)
	})
}

// dispatch authorizes the caller and answers with external for a
// connected-account message, or with hosted for a stored one the caller
// may read.
func (route InboxEmailRoute) dispatch(id string, request *http.Request, external func(RouteCaller, string) *http.Response, hosted func(RouteCaller, string, map[string]any) *http.Response) *http.Response {
	caller, refusal := route.signed(id, request)
	if refusal != nil {
		return refusal
	}
	if (ExternalMessage{}).IsRoute(id) {
		return external(caller, id)
	}
	documentId, email, refusal := route.Parts.Owned(caller, id)
	if refusal != nil {
		return refusal
	}
	return hosted(caller, documentId, email)
}

// signed answers the caller, or the refusal when the request has no session
// or names no message.
func (route InboxEmailRoute) signed(id string, request *http.Request) (RouteCaller, *http.Response) {
	caller, refusal := route.Parts.Access.Signed(request)
	if refusal == nil && id == "" {
		refusal = Answer{}.NotFound("Email not found")
	}
	return caller, refusal
}

func (route InboxEmailRoute) readExternal(caller RouteCaller, id string) *http.Response {
	provider := route.Parts.Provider()
	loaded, err := provider.Load(caller.Token, id, true)
	if err != nil {
		return provider.Accounts.Failure(err)
	}
	if loaded == nil {
		return Answer{}.NotFound("Email not found")
	}
	detail := ExternalMessage{}.Detail(loaded.Indexed, loaded.RawBase64)
	if detail == nil {
		return Answer{}.NotFound("Email not found")
	}
	detail["calendarInvite"] = nil
	return Answer{}.Ok(detail)
}

func (route InboxEmailRoute) detail(caller RouteCaller, id string, email map[string]any) (map[string]any, error) {
	parts := route.Parts
	presented, err := MigratedRecipients{Migrations: parts.Migrations}.PresentedOne(email)
	if err != nil {
		return nil, err
	}
	source, err := parts.Source(id)
	if err != nil {
		return nil, err
	}
	records, err := parts.AttachmentRecords(id)
	if err != nil {
		return nil, err
	}
	invites := CalendarInvite{Events: parts.Calendar, Attachments: parts.Attachments}
	selected := invites.Best(records)
	invite, err := invites.Find(caller.Email, id, email, selected)
	if err != nil {
		return nil, err
	}
	if invite == nil {
		selected = nil
	}
	visible := []map[string]any{}
	for _, record := range invites.Visible(records, selected) {
		summary := map[string]any{}
		for _, field := range []string{"id", "filename", "contentType", "size", "disposition", "contentId"} {
			if value, found := record[field]; found {
				summary[field] = value
			}
		}
		visible = append(visible, summary)
	}
	detail := route.withGitHub(presented)
	detail["hasRawSource"] = source != nil
	detail["attachments"] = visible
	detail["calendarInvite"] = nil
	if invite != nil {
		detail["calendarInvite"] = invite
	}
	return route.presented(caller, detail)
}

// withGitHub answers a copy of email with its GitHub notification target,
// when it has one.
func (InboxEmailRoute) withGitHub(email map[string]any) map[string]any {
	copied := TextValue{}.Copy(email)
	if target := (GitHubNotification{}).Target(email); target != "" {
		copied["githubUrl"] = target
	}
	return copied
}

func (route InboxEmailRoute) presented(caller RouteCaller, email map[string]any) (map[string]any, error) {
	mappings, err := route.Parts.Mappings(caller)
	return mappings.PresentEmail(email), err
}

// Change applies a read, starred or folder patch and answers the message as
// it now reads; a connected account queues the change at the provider.
func (route InboxEmailRoute) Change(id string, request *http.Request) *http.Response {
	caller, refusal := route.signed(id, request)
	if refusal != nil {
		return refusal
	}
	body, refusal := RequestBody{}.Value(request)
	if refusal != nil {
		return refusal
	}
	patch, err := EmailPatch{}.Parsed(body)
	if err != nil {
		return Answer{}.BadRequest(err.Error())
	}
	if (ExternalMessage{}).IsRoute(id) {
		return route.changeExternal(caller, id, patch)
	}
	documentId, email, refusal := route.Parts.Owned(caller, id)
	if refusal != nil {
		return refusal
	}
	updated, err := route.changed(caller, id, documentId, email, patch)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(updated)
}

func (route InboxEmailRoute) changed(caller RouteCaller, id string, documentId string, email map[string]any, patch map[string]any) (map[string]any, error) {
	parts := route.Parts
	applied := TextValue{}.Copy(patch)
	if folder, isText := applied["folder"].(string); isText {
		mappings, err := parts.Mappings(caller)
		if err != nil {
			return nil, err
		}
		if path := mappings.PathFor(TextValue{}.Of(email["domain"]), folder); path != "" {
			applied["folder"] = path
		}
	}
	if err := parts.Update(documentId, applied); err != nil {
		return nil, err
	}
	updated := TextValue{}.Copy(email)
	for field, value := range applied {
		updated[field] = value
	}
	updated = route.withGitHub(updated)
	source, err := parts.Source(id)
	if err != nil {
		return nil, err
	}
	attachments, err := parts.AttachmentSummaries(id)
	if err != nil {
		return nil, err
	}
	updated["hasRawSource"] = source != nil
	updated["attachments"] = attachments
	return route.presented(caller, updated)
}

func (route InboxEmailRoute) changeExternal(caller RouteCaller, id string, patch map[string]any) *http.Response {
	provider := route.Parts.Provider()
	loaded, err := provider.Load(caller.Token, id, true)
	if loaded == nil && err == nil {
		return Answer{}.NotFound("Email not found")
	}
	var mutations []map[string]any
	if err == nil {
		mutations, err = provider.QueuePatch(caller.Token, loaded, patch, "")
	}
	var folderError *ExternalFolderError
	if errors.As(err, &folderError) {
		return Answer{}.BadRequest(folderError.Error())
	}
	if err != nil {
		return provider.Accounts.Failure(err)
	}
	projected := provider.Projected(loaded, patch)
	detail := ExternalMessage{}.Detail(projected.Indexed, projected.RawBase64)
	if detail == nil {
		return Answer{}.NotFound("Email not found")
	}
	ids := []any{}
	for _, mutation := range mutations {
		ids = append(ids, mutation["id"])
	}
	detail["calendarInvite"] = nil
	detail["pendingRemoteMutation"] = true
	detail["mutationIds"] = ids
	return Answer{}.Ok(detail)
}

// Remove deletes the message; a connected account queues the delete at the
// provider.
func (route InboxEmailRoute) Remove(id string, request *http.Request) *http.Response {
	return route.dispatch(id, request, route.removeExternal, func(_ RouteCaller, documentId string, _ map[string]any) *http.Response {
		if err := route.Parts.Delete(documentId, id); err != nil {
			return RouteAccess{}.Failure(err)
		}
		return Answer{}.Ok(map[string]any{"deleted": id})
	})
}

func (route InboxEmailRoute) removeExternal(caller RouteCaller, id string) *http.Response {
	provider := route.Parts.Provider()
	loaded, err := provider.Load(caller.Token, id, true)
	if err != nil {
		return provider.Accounts.Failure(err)
	}
	if loaded == nil {
		return Answer{}.NotFound("Email not found")
	}
	mutation, err := provider.QueueDelete(caller.Token, loaded)
	if err != nil {
		return provider.Accounts.Failure(err)
	}
	return Answer{}.Ok(map[string]any{"deleted": id, "pendingRemoteMutation": true, "mutationId": TextValue{}.Or(mutation["id"], "")})
}
