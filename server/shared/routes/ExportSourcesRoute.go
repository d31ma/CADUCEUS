import (
	"errors"
	"net/http"
	"strings"
)

// ExportSourcesRoute answers GET /inbox/export/rfc822: the exact RFC 822
// sources of one authorized hosted folder, or of one connected-account
// folder, as source frames.
//
// The JavaScript route streamed one message at a time. A Go route answers
// once, so the frames are assembled before the answer is sent.
type ExportSourcesRoute struct {
	Parts MailboxParts
}

func (ExportSourcesRoute) pageSize() int { return 100 }

func (ExportSourcesRoute) lastOffset() int { return 100_000 }

// Export answers the folder's frames.
func (route ExportSourcesRoute) Export(request *http.Request) *http.Response {
	caller, refusal := route.Parts.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	query := MailboxQuery{}.Of(request)
	accountId := TextValue{}.Trim(query["account"])
	frames := &SourceFrames{}
	if accountId != "" {
		folder, err := frames.ProviderFolder(query["folder"])
		if err != nil {
			return Answer{}.BadRequest(err.Error())
		}
		return route.provider(caller, accountId, folder, frames)
	}
	folder, err := frames.HostedFolder(query["folder"])
	if err != nil {
		return Answer{}.BadRequest(err.Error())
	}
	domain := strings.ToLower(TextValue{}.Trim(TextValue{}.Of(TextValue{}.Or(query["domain"], caller.Domain))))
	if domain == "" {
		return Answer{}.BadRequest("domain is required")
	}
	if domain != caller.Domain {
		if !(RouteAccess{}).domainPattern().MatchString(domain) {
			return Answer{}.Forbidden("Access denied")
		}
		if _, refusal := route.Parts.Access.Allowed(request, (RouteAccess{}).DomainRead(), domain, ""); refusal != nil {
			return refusal
		}
	}
	return route.hosted(caller, domain, folder, frames)
}

func (route ExportSourcesRoute) hosted(caller RouteCaller, domain string, folder string, frames *SourceFrames) *http.Response {
	mappings, err := route.Parts.Mappings(caller)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if path := mappings.PathFor(domain, folder); path != "" {
		folder = path
	}
	emails := route.Parts.Folder([]string{domain}, folder, caller.Email)
	for {
		email, err := emails.Next()
		if err == nil && email == nil {
			return frames.Response()
		}
		var source []byte
		if err == nil {
			source, err = route.source(email, domain)
		}
		if err == nil {
			err = frames.Add(email, source)
		}
		if err != nil {
			return Answer{}.Unavailable(err.Error())
		}
	}
}

func (route ExportSourcesRoute) source(email map[string]any, domain string) ([]byte, error) {
	id := TextValue{}.Of(email["id"])
	record, err := route.Parts.Source(id)
	if err != nil {
		return nil, err
	}
	if record == nil || record["domain"] != domain {
		return nil, errors.New("RFC 822 source is unavailable for message " + id)
	}
	return route.Parts.SourceBytes(record)
}

func (route ExportSourcesRoute) provider(caller RouteCaller, accountId string, folder string, frames *SourceFrames) *http.Response {
	provider := route.Parts.Provider()
	response, err := provider.Accounts.Folders(caller.Token, accountId)
	if err != nil {
		return provider.Accounts.Failure(err)
	}
	target := provider.ResolveFolder(TextValue{}.List(response["folders"]), folder)
	if target == nil {
		return Answer{}.NotFound("Connected mailbox folder not found")
	}
	for offset := 0; offset <= route.lastOffset(); offset += route.pageSize() {
		page, err := provider.Accounts.UnifiedMessages(caller.Token, map[string]any{
			"accountId": accountId, "folderStableId": target["stableId"], "limit": route.pageSize(), "offset": offset,
		})
		if err != nil {
			return provider.Accounts.Failure(err)
		}
		messages := TextValue{}.List(page["messages"])
		for _, item := range messages {
			if err := route.providerFrame(caller, provider, accountId, item, frames); err != nil {
				return Answer{}.Unavailable(err.Error())
			}
		}
		if len(messages) < route.pageSize() {
			return frames.Response()
		}
	}
	return Answer{}.Unavailable("Connected mailbox export exceeds the pagination limit")
}

func (ExportSourcesRoute) providerFrame(caller RouteCaller, provider UnifiedMail, accountId string, item any, frames *SourceFrames) error {
	message, _ := item.(map[string]any)
	stableId := TextValue{}.Of(TextValue{}.Or(message["stableId"], ""))
	record, err := provider.Accounts.MessageSource(caller.Token, accountId, stableId)
	if err != nil {
		return err
	}
	source := Base64Text{}.Decoded(TextValue{}.Of(TextValue{}.Or(record["rawRFC822Base64"], "")))
	if len(source) == 0 {
		return errors.New("RFC 822 source is unavailable for message " + TextValue{}.Of(message["stableId"]))
	}
	return frames.Add(map[string]any{"id": message["stableId"], "sender": message["sender"], "subject": message["subject"], "receivedAt": message["receivedAt"]}, source)
}
