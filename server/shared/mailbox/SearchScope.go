import (
	"errors"
	"math"
	"strings"
)

// SearchScope validates the query shared by GET /search and GET /threads:
// the search text (at most 2,000 characters), one hosted domain or one
// connected account, and the result limit (1 to 1,000).
type SearchScope struct {
	Raw       string
	Domain    string
	AccountId string
	Limit     int
	// Status is the HTTP status of an invalid scope (0 when it is valid).
	Status int
}

func (SearchScope) longestQuery() int { return 2_000 }

func (SearchScope) mostResults() int { return 1_000 }

// Parsed answers the scope of a query for a caller of the given domains,
// with the refusal message when it is invalid.
func (scope SearchScope) Parsed(query map[string]string, callerDomains []string) (SearchScope, error) {
	text := TextValue{}
	parsed := SearchScope{Raw: text.Trim(query["q"]), Domain: strings.ToLower(text.Trim(query["domain"])), AccountId: text.Trim(query["account"])}
	parsed.Status = 400
	if text.Length(parsed.Raw) > scope.longestQuery() {
		return parsed, errors.New("Search queries are limited to 2,000 characters")
	}
	if parsed.Domain != "" && !text.Includes(callerDomains, parsed.Domain) {
		parsed.Status = 403
		return parsed, errors.New("Mailbox is outside the authenticated account scope")
	}
	if parsed.AccountId != "" && !(ExternalMessage{}).IsAccountId(parsed.AccountId) {
		return parsed, errors.New("Invalid connected account identifier")
	}
	if parsed.Domain != "" && parsed.AccountId != "" {
		return parsed, errors.New("Choose only one mailbox domain or connected account")
	}
	parsed.Limit = scope.mostResults()
	if value := query["limit"]; value != "" {
		limit := RecordValues{}.Numeric(value)
		if math.IsNaN(limit) || limit != math.Trunc(limit) || limit < 1 || limit > float64(scope.mostResults()) {
			return parsed, errors.New("limit must be an integer from 1 to 1,000")
		}
		parsed.Limit = int(limit)
	}
	parsed.Status = 0
	return parsed, nil
}

// Allowed answers the domains the search reads: the named one, or every
// domain of the caller.
func (scope SearchScope) Allowed(callerDomains []string) []string {
	if scope.Domain != "" {
		return []string{scope.Domain}
	}
	return callerDomains
}

// Searchable reports whether an email takes part in a search: trash and
// spam only when the search names a category or is empty.
func (scope SearchScope) Searchable(email map[string]any, expression *SearchExpression) bool {
	if scope.Raw == "" || expression.HasField("in") {
		return true
	}
	text := TextValue{}
	folder := strings.ToLower(text.Of(text.Or(email["systemFolder"], text.Or(email["folder"], "inbox"))))
	return folder != "trash" && folder != "spam"
}
