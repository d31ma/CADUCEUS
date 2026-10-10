import (
	"math"
	"net/http"
	"strings"
	"time"
)

// ThreadsRoute answers GET /threads: the caller's hosted and connected mail
// grouped into conversations, for one folder, one search or the one
// conversation holding a selected email.
type ThreadsRoute struct {
	Parts MailboxParts
}

func (ThreadsRoute) largestConversationSource() int { return 25 * 1024 * 1024 }

// List answers the conversations.
func (route ThreadsRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Parts.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	scope, refusal := route.Parts.Scope(caller, request)
	if refusal != nil {
		return refusal
	}
	query := RouteQuery{}.Of(request)
	allowed := scope.Allowed(caller.Domains)
	messages, refusal := route.messages(caller, scope, allowed)
	if refusal != nil {
		return refusal
	}
	threads := ThreadsRecords{}.GroupMessagesIntoThreads(messages, caller.Email)
	if query["emailId"] != "" {
		return route.selected(caller, threads, query["emailId"])
	}
	if scope.Raw != "" {
		return route.searched(threads, scope, allowed)
	}
	folder := query["folder"]
	if folder == "" {
		folder = "inbox"
	}
	mappings, err := route.Parts.Mappings(caller)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	folderByDomain := mappings.FolderFilter(allowed, folder)
	kept := []Thread{}
	for _, thread := range threads {
		if len(kept) < scope.Limit && route.inFolder(thread, folder, folderByDomain) {
			kept = append(kept, thread)
		}
	}
	return Answer{}.Ok(kept)
}

// messages loads each store through its own owner boundary before any
// conversation edges form.
func (route ThreadsRoute) messages(caller RouteCaller, scope SearchScope, allowed []string) ([]map[string]any, *http.Response) {
	provider := route.Parts.Provider()
	stored := []map[string]any{}
	var err error
	if scope.AccountId == "" {
		stored, err = route.Parts.Listed(allowed, EmailListFilters{Folder: "all", OwnerEmail: caller.Email})
	}
	external := []map[string]any{}
	if err == nil && scope.Domain == "" {
		external, err = provider.Summaries(caller.Token, SummaryFilter{AccountId: scope.AccountId})
	}
	hosted := []map[string]any{}
	if err == nil {
		hosted, err = route.Parts.Presented(caller, stored)
	}
	if err != nil {
		return nil, provider.Accounts.Failure(err)
	}
	return append(hosted, external...), nil
}

func (route ThreadsRoute) inFolder(thread Thread, folder string, folderByDomain map[string]string) bool {
	text := TextValue{}
	wanted := strings.ToLower(folder)
	for _, email := range thread.Emails {
		logical := strings.ToLower(text.Of(text.Or(email["systemFolder"], text.Or(email["folder"], "inbox"))))
		if logical == wanted {
			return true
		}
		if text.Truthy(email["external"]) {
			continue
		}
		physical := text.Or(folderByDomain[text.Of(email["domain"])], wanted)
		if strings.ToLower(text.Of(text.Or(email["folder"], "inbox"))) == strings.ToLower(text.Of(physical)) {
			return true
		}
	}
	return false
}

func (route ThreadsRoute) searched(threads []Thread, scope SearchScope, allowed []string) *http.Response {
	attachments, err := route.Parts.AttachmentsByEmail(allowed)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	expression := SearchParser{}.WithFallback(scope.Raw)
	matching := []Thread{}
	for _, thread := range threads {
		if len(matching) >= scope.Limit {
			break
		}
		for _, email := range thread.Emails {
			summaries := attachments[TextValue{}.Of(email["id"])]
			if (TextValue{}).Truthy(email["external"]) {
				summaries = ExternalMessage{}.AttachmentSummaries(email)
			}
			if scope.Searchable(email, expression) && (SearchMatcher{Email: email, Attachments: summaries, Now: time.Now()}).Matches(expression) {
				thread.LatestEmail = thread.Emails[len(thread.Emails)-1]
				matching = append(matching, thread)
				break
			}
		}
	}
	return Answer{}.Ok(matching)
}

// selected answers the one conversation holding the email, with hosted
// attachment summaries and the provider messages' bodies, within a bounded
// total of provider source.
func (route ThreadsRoute) selected(caller RouteCaller, threads []Thread, emailId string) *http.Response {
	thread := ThreadsRecords{}.Holding(threads, emailId)
	if thread == nil {
		return Answer{}.Ok([]any{})
	}
	attachments, err := route.Parts.AttachmentsByEmail(caller.Domains)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	provider := route.Parts.Provider()
	remaining := route.largestConversationSource()
	emails := []map[string]any{}
	for _, email := range thread.Emails {
		if !(TextValue{}).Truthy(email["external"]) {
			emails = append(emails, route.withAttachments(email, attachments[TextValue{}.Of(email["id"])]))
			continue
		}
		detail, used, err := route.externalDetail(caller, provider, email, remaining)
		if err != nil {
			return provider.Accounts.Failure(err)
		}
		if detail != nil {
			remaining -= used
			emails = append(emails, detail)
		}
	}
	presented := *thread
	presented.Count = len(emails)
	presented.Emails = emails
	if len(emails) > 0 {
		presented.LatestEmail = emails[len(emails)-1]
	}
	return Answer{}.Ok([]Thread{presented})
}

func (ThreadsRoute) withAttachments(email map[string]any, attachments []map[string]any) map[string]any {
	copied := TextValue{}.Copy(email)
	copied["attachments"] = attachments
	if attachments == nil {
		copied["attachments"] = []map[string]any{}
	}
	return copied
}

// externalDetail answers a provider message's reader view and the source
// bytes it used, or the truncated summary once the bound is reached; nil
// when the message is gone.
func (route ThreadsRoute) externalDetail(caller RouteCaller, provider UnifiedMail, email map[string]any, remaining int) (map[string]any, int, error) {
	truncated := route.withAttachments(email, ExternalMessage{}.AttachmentSummaries(email))
	truncated["body"] = TextValue{}.Or(email["snippet"], "")
	truncated["conversationBodyTruncated"] = true
	if int(math.Floor(TextValue{}.Number(email["rawSize"]))) > remaining {
		return truncated, 0, nil
	}
	loaded, err := provider.Load(caller.Token, TextValue{}.Of(email["id"]), true)
	if err != nil || loaded == nil {
		return nil, 0, err
	}
	encoded, _ := loaded.RawBase64.(string)
	used := int(math.Floor(float64(len(encoded)) * 0.75))
	if used > remaining {
		return truncated, 0, nil
	}
	detail := ExternalMessage{}.Detail(loaded.Indexed, loaded.RawBase64)
	if detail == nil {
		return nil, 0, nil
	}
	return detail, used, nil
}
