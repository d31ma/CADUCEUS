import "net/http"

// InboxRoute answers GET /inbox: the caller's hosted mail merged with their
// connected-account mail, newest first.
type InboxRoute struct {
	Parts MailboxParts
}

// List answers the filtered mailbox listing.
func (route InboxRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Parts.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	filters, err := InboxFilters{}.Parsed(RouteQuery{}.Of(request))
	if err != nil {
		return Answer{}.BadRequest(err.Error())
	}
	if filters.Domain != "" && filters.AccountId != "" {
		return Answer{}.BadRequest("Choose only one mailbox domain or connected account")
	}
	if filters.Domain != "" && !(TextValue{}).Includes(caller.Domains, filters.Domain) {
		return Answer{}.Forbidden("Mailbox is outside the authenticated account scope")
	}
	hosted, err := route.hosted(caller, filters)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	external := []map[string]any{}
	if filters.Domain == "" {
		external, err = route.Parts.Provider().Summaries(caller.Token, SummaryFilter{
			AccountId: filters.AccountId, FolderRole: filters.Folder, Query: filters.Query, Read: filters.Read, Starred: filters.Starred,
		})
		if err != nil {
			return route.Parts.Provider().Accounts.Failure(err)
		}
	}
	return Answer{}.Ok(ExternalMessage{}.Merged(hosted, route.withAttachments(external, filters.HasAttachment), route.offset(filters), route.limit(filters)))
}

// hosted answers the caller's presented hosted mail. Due follow-up reminders
// surface first, so they resurface even with no browser connected.
func (route InboxRoute) hosted(caller RouteCaller, filters InboxFilters) ([]map[string]any, error) {
	parts := route.Parts
	if _, err := (DueFollowUps{Reminders: parts.Reminders, Emails: parts.Emails, Categories: parts.Categories}).Process(caller.Email, (EpochTime{}).Now()); err != nil {
		return nil, err
	}
	allowed := caller.Domains
	if filters.Domain != "" {
		allowed = []string{filters.Domain}
	}
	listing := EmailListFilters{
		Query: filters.Query, Folder: filters.Folder, Read: filters.Read, Starred: filters.Starred,
		HasAttachment: filters.HasAttachment, OwnerEmail: caller.Email,
	}
	if filters.Folder != "" && filters.AccountId == "" {
		mappings, err := parts.Mappings(caller)
		if err != nil {
			return nil, err
		}
		listing.FolderByDomain = mappings.FolderFilter(allowed, filters.Folder)
	}
	emails := []map[string]any{}
	if filters.AccountId == "" {
		var err error
		if emails, err = parts.Listed(allowed, listing); err != nil {
			return nil, err
		}
	}
	return parts.Presented(caller, emails)
}

func (InboxRoute) withAttachments(external []map[string]any, hasAttachment *bool) []map[string]any {
	if hasAttachment == nil {
		return external
	}
	kept := []map[string]any{}
	for _, message := range external {
		if message["hasAttachments"] == *hasAttachment {
			kept = append(kept, message)
		}
	}
	return kept
}

func (InboxRoute) offset(filters InboxFilters) int {
	if filters.Offset == nil {
		return 0
	}
	return *filters.Offset
}

func (InboxRoute) limit(filters InboxFilters) int {
	if filters.Limit == nil {
		return -1
	}
	return *filters.Limit
}
