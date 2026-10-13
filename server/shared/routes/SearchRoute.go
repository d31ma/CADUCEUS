import (
	"net/http"
	"time"
)

// SearchRoute answers GET /search: full-text search with the structured
// grammar (from:, to:, subject:, body:, in:, is:, has:, before:, after:,
// quoted phrases, OR, NOT and parentheses) over hosted and connected mail.
type SearchRoute struct {
	Parts MailboxParts
}

// Find answers the matching emails, newest first.
func (route SearchRoute) Find(request *http.Request) *http.Response {
	caller, refusal := route.Parts.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	scope, refusal := route.Parts.Scope(caller, request)
	if refusal != nil {
		return refusal
	}
	allowed := scope.Allowed(caller.Domains)
	stored := []map[string]any{}
	var err error
	if scope.AccountId == "" {
		if stored, err = route.Parts.Listed(allowed, EmailListFilters{OwnerEmail: caller.Email}); err != nil {
			return RouteAccess{}.Failure(err)
		}
	}
	attachments, err := route.Parts.AttachmentsByEmail(allowed)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	presented, err := route.Parts.Presented(caller, stored)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	external := []map[string]any{}
	if scope.Domain == "" {
		if external, err = route.Parts.Provider().Summaries(caller.Token, SummaryFilter{AccountId: scope.AccountId}); err != nil {
			return route.Parts.Provider().Accounts.Failure(err)
		}
	}
	expression := SearchParser{}.WithFallback(scope.Raw)
	hosted := []map[string]any{}
	for _, email := range presented {
		matcher := SearchMatcher{Email: email, Attachments: attachments[TextValue{}.Of(email["id"])], Now: time.Now()}
		if scope.Searchable(email, expression) && matcher.Matches(expression) {
			hosted = append(hosted, email)
		}
	}
	matched := []map[string]any{}
	for _, email := range external {
		matcher := SearchMatcher{Email: email, Attachments: ExternalMessage{}.AttachmentSummaries(email), Now: time.Now()}
		if scope.Searchable(email, expression) && matcher.Matches(expression) {
			matched = append(matched, email)
		}
	}
	return Answer{}.Ok(ExternalMessage{}.Merged(hosted, matched, 0, scope.Limit))
}
