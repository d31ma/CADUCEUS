import (
	"net/http"
	"regexp"
	"strings"
)

// GithubProfileRoute answers GET /github/profile?email=: best-effort public
// GitHub enrichment, off by default, and only for an exact contact of the
// caller or a participant in mail the caller owns.
type GithubProfileRoute struct {
	Access   *AccessHelper
	Contacts *ContactsHelper
	Emails   *EmailsHelper
	GitHub   *GitHubProfilesService
	Users    *UsersHelper
}

// Find answers the enrichment of one address.
func (route GithubProfileRoute) Find(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	requested := ""
	if values := request.URL.Query()["email"]; len(values) > 0 {
		requested = values[len(values)-1]
	}
	email := Security{}.NormalizeEmailAddress(requested)
	if email == "" {
		return Answer{}.BadRequest("email must be a valid email address")
	}
	_, user, err := route.Users.FindByEmail(caller.Email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if user == nil {
		return Answer{}.NotFound("User not found")
	}
	if user["githubProfileEnrichment"] != true {
		return Answer{}.Ok(map[string]any{"enabled": false, "status": "disabled"})
	}
	related, err := route.related(caller, email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if !related {
		return Answer{}.NotFound("Contact not found")
	}
	result := route.GitHub.FindByEmail(email)
	result["enabled"] = true
	return Answer{}.Ok(result)
}

// related reports whether the address is the caller's contact or an exact
// participant of mail the caller owns, so body text never authorizes it.
func (route GithubProfileRoute) related(caller RouteCaller, email string) (bool, error) {
	contact, err := route.Contacts.FindByEmail(caller.Email, email)
	if err != nil || contact != nil {
		return contact != nil, err
	}
	limit := 20
	candidates, err := route.Emails.List(caller.Domains, EmailListFilters{OwnerEmail: caller.Email, Query: email, Limit: &limit})
	if err != nil {
		return false, err
	}
	for _, message := range candidates {
		if (PreferenceChoices{}).Includes(route.participants(message), email) {
			return true, nil
		}
	}
	return false, nil
}

func (GithubProfileRoute) participants(message map[string]any) []string {
	values := []any{message["sender"], message["recipient"], message["originalRecipient"]}
	for _, field := range []string{"to", "cc", "bcc"} {
		if list, isList := message[field].([]any); isList {
			values = append(values, list...)
		}
		if list, isList := message[field].([]string); isList {
			for _, item := range list {
				values = append(values, item)
			}
		}
	}
	pattern := regexp.MustCompile("(?i)[A-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?(?:\\.[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?)+")
	addresses := []string{}
	for _, value := range values {
		for _, address := range pattern.FindAllString(TextValue{}.Of(RecordValues{}.Or(value, "")), -1) {
			if lower := strings.ToLower(address); !(PreferenceChoices{}).Includes(addresses, lower) {
				addresses = append(addresses, lower)
			}
		}
	}
	return addresses
}
