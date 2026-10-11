import "net/http"

// ContactGroupsRoute answers /contacts/groups and /contacts/groups/{id}:
// the caller's contact groups.
type ContactGroupsRoute struct {
	Access   *AccessHelper
	Contacts *ContactsService
}

// List answers the caller's groups sorted by name.
func (route ContactGroupsRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	groups, err := route.Contacts.ListGroups(caller.Email)
	return ContactsRoute{}.answered(groups, err)
}

// Create stores a new group.
func (route ContactGroupsRoute) Create(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	group, err := route.Contacts.CreateGroup(caller.Email, body["name"])
	if err != nil {
		return Answer{}.BadRequest(err.Error())
	}
	return Answer{}.Ok(group)
}

// Update renames one of the caller's groups.
func (route ContactGroupsRoute) Update(id string, request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	group, err := route.Contacts.UpdateGroup(id, caller.Email, body["name"])
	if err != nil {
		return Answer{}.BadRequest(err.Error())
	}
	if group == nil {
		return Answer{}.NotFound("Contact group not found")
	}
	return Answer{}.Ok(group)
}

// Delete deletes one of the caller's groups and takes it off its contacts.
func (route ContactGroupsRoute) Delete(id string, request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	deleted, err := route.Contacts.DeleteGroup(id, caller.Email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if !deleted {
		return Answer{}.NotFound("Contact group not found")
	}
	return Answer{}.Ok(map[string]any{"deleted": true})
}
