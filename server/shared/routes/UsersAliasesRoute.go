import (
	"net/http"
	"strings"
)

// UsersAliasesRoute answers /users/aliases: the extra addresses of the
// caller's own mailbox.
type UsersAliasesRoute struct {
	Access *AccessHelper
	Users  *UsersHelper
}

// Add adds one alias in a domain assigned to the caller.
func (route UsersAliasesRoute) Add(request *http.Request) *http.Response {
	body, caller, refusal := route.signed(request)
	if refusal != nil {
		return refusal
	}
	alias := Security{}.NormalizeEmailAddress(BodyText{}.String(body["alias"]))
	if alias == "" {
		return Answer{}.BadRequest("alias invalid")
	}
	aliasDomain := strings.Split(alias, "@")[1]
	if !(Security{}).HasDomainClaim(caller.Domains, aliasDomain) {
		return Answer{}.Forbidden("Domain access denied")
	}
	documentId, user, refusal := route.user(caller)
	if refusal != nil {
		return refusal
	}
	if !(PreferenceChoices{}).Includes(TextList{}.Of(user["domains"]), aliasDomain) {
		return Answer{}.Forbidden("Alias domain is not assigned to this user")
	}
	if alias == user["email"] {
		return Answer{}.BadRequest("alias must not match primary email")
	}
	existingId, _, err := route.Users.FindByEmail(alias)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if existingId != "" && existingId != documentId {
		return Answer{}.BadRequest("alias already belongs to another user")
	}
	return route.store(documentId, user, TextList{}.Distinct(append(TextList{}.Of(user["aliases"]), alias)))
}

// Remove removes one alias of the caller.
func (route UsersAliasesRoute) Remove(request *http.Request) *http.Response {
	body, caller, refusal := route.signed(request)
	if refusal != nil {
		return refusal
	}
	alias := Security{}.NormalizeEmailAddress(BodyText{}.String(body["alias"]))
	if alias == "" {
		return Answer{}.BadRequest("alias invalid")
	}
	documentId, user, refusal := route.user(caller)
	if refusal != nil {
		return refusal
	}
	current := TextList{}.Of(user["aliases"])
	aliases := []string{}
	for _, item := range current {
		if item != alias {
			aliases = append(aliases, item)
		}
	}
	if len(aliases) == len(current) {
		return Answer{}.NotFound("Alias not found")
	}
	return route.store(documentId, user, aliases)
}

// List answers the caller's aliases.
func (route UsersAliasesRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	_, user, err := route.Users.FindByEmail(caller.Email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if user == nil {
		return Answer{}.NotFound("User not found")
	}
	return Answer{}.Ok(map[string]any{"email": user["email"], "aliases": RecordValues{}.Coalesce(user["aliases"], []any{})})
}

func (route UsersAliasesRoute) signed(request *http.Request) (map[string]any, RouteCaller, *http.Response) {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return nil, RouteCaller{}, refusal
	}
	caller, refusal := route.Access.Signed(request)
	return body, caller, refusal
}

func (route UsersAliasesRoute) user(caller RouteCaller) (string, map[string]any, *http.Response) {
	documentId, user, err := route.Users.FindByEmail(caller.Email)
	if err != nil {
		return "", nil, RouteAccess{}.Failure(err)
	}
	if documentId == "" || user == nil {
		return "", nil, Answer{}.NotFound("User not found")
	}
	return documentId, user, nil
}

func (route UsersAliasesRoute) store(documentId string, user map[string]any, aliases []string) *http.Response {
	items := make([]any, len(aliases))
	for index, alias := range aliases {
		items[index] = alias
	}
	email := RecordValues{}.Text(user["email"])
	if err := route.Users.UpdateAliases(documentId, items, email); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(map[string]any{"email": email, "aliases": aliases})
}
