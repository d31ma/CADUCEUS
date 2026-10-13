import "net/http"

// TemplatesRoute answers /templates and /templates/{id}: the caller's own
// message templates.
type TemplatesRoute struct {
	Access    *AccessHelper
	Templates *TemplatesHelper
}

// List answers the caller's templates by name.
func (route TemplatesRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxRead(), "", "")
	if refusal != nil {
		return refusal
	}
	records, err := route.Templates.ListForOwner(caller.Email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	entries := ContentRecords{}.ByName(records)
	presented := make([]any, len(entries))
	for index, entry := range entries {
		presented[index] = route.present(entry)
	}
	return Answer{}.Ok(presented)
}

// Create stores a new template with a name unique among the caller's.
func (route TemplatesRoute) Create(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Value(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxWrite(), "", "")
	if refusal != nil {
		return refusal
	}
	input, problem := TemplateContent{}.Normalized(body, false)
	if problem != "" {
		return Answer{}.BadRequest(problem)
	}
	id := ContentRecords{}.NewId("tpl")
	existing, err := route.Templates.ListForOwner(caller.Email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	now := EpochTime{}.Now()
	entry := ContentRecords{}.Merged(input, map[string]any{
		"id":        id,
		"name":      ContentNames{}.Unique(TextValue{}.Of(input["name"]), existing, ""),
		"owner":     caller.Email,
		"createdAt": now,
		"updatedAt": now,
	})
	if _, err := route.Templates.Put(entry); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(route.present(entry))
}

// DeleteNamed deletes the caller's template the body's id names.
func (route TemplatesRoute) DeleteNamed(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxWrite(), "", "")
	if refusal != nil {
		return refusal
	}
	id, isText := body["id"].(string)
	if !isText || id == "" {
		return Answer{}.BadRequest("id is required")
	}
	return route.deleted(caller, id, true)
}

// Update changes the fields the body names of one of the caller's
// templates.
func (route TemplatesRoute) Update(id string, request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Value(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxWrite(), "", "")
	if refusal != nil {
		return refusal
	}
	if id == "" {
		return Answer{}.NotFound("Template not found")
	}
	patch, problem := TemplateContent{}.Normalized(body, true)
	if problem != "" {
		return Answer{}.BadRequest(problem)
	}
	documentId, existing, refusal := route.owned(caller, id)
	if refusal != nil {
		return refusal
	}
	return route.content().Revised(caller, documentId, existing, patch, route.present)
}

// Delete deletes one of the caller's templates by path.
func (route TemplatesRoute) Delete(id string, request *http.Request) *http.Response {
	return route.content().Remove(route.Access, request, id, map[string]any{"deleted": id})
}

// deleted deletes an owned template and answers {deleted: true} for the
// collection route or {deleted: id} for the item route.
func (route TemplatesRoute) deleted(caller RouteCaller, id string, flag bool) *http.Response {
	removed := map[string]any{"deleted": id}
	if flag {
		removed["deleted"] = true
	}
	return route.content().Removed(caller, id, removed)
}

func (route TemplatesRoute) owned(caller RouteCaller, id string) (string, map[string]any, *http.Response) {
	return route.content().Lookup(caller, id)
}

func (route TemplatesRoute) content() OwnedContent {
	return OwnedContent{Find: route.Templates.FindById, List: route.Templates.ListForOwner, Update: route.Templates.Update, Delete: route.Templates.Delete, Missing: "Template not found"}
}

func (TemplatesRoute) present(entry map[string]any) map[string]any {
	return ContentRecords{}.Viewed(entry, "id", "name", "subject", "text", "to", "cc")
}
