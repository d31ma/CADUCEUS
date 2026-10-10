import "net/http"

// CategoriesRoute answers /categories: the owner's physical folders and
// labels per exact mailbox, with their system roles.
type CategoriesRoute struct {
	Parts MailboxParts
}

// List answers the roles and the owner's accounts.
func (route CategoriesRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Parts.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	parts := route.Parts
	accounts, err := parts.CategoryAccounts(caller)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if accounts == nil {
		return Answer{}.NotFound("User not found")
	}
	return Answer{}.Ok(map[string]any{"roles": CategoryMappings{}.Roles(), "accounts": accounts})
}

// Create adds a nested folder or label to the owner's own catalog.
func (route CategoriesRoute) Create(request *http.Request) *http.Response {
	caller, refusal := route.Parts.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	body, refusal := RequestBody{}.Value(request)
	if refusal != nil {
		return refusal
	}
	category, problem, err := route.Parts.CreateCategory(caller, body)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	switch {
	case problem == nil:
		return Answer{}.Ok(map[string]any{"category": category})
	case problem.NotFound:
		return Answer{}.NotFound(problem.Message)
	case problem.Forbidden:
		return Answer{}.Forbidden(problem.Message)
	}
	return Answer{}.BadRequest(problem.Message)
}
