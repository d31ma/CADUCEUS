import (
	"net/http"
	"strings"
)

// SignaturesRoute answers /signatures and /signatures/{id}: the caller's
// own message signatures.
type SignaturesRoute struct {
	Access     *AccessHelper
	Signatures *SignaturesHelper
	Users      *UsersHelper
}

// List answers the caller's signatures by name.
func (route SignaturesRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxRead(), "", "")
	if refusal != nil {
		return refusal
	}
	records, err := route.Signatures.ListForOwner(caller.Email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	presented := []any{}
	for _, entry := range (ContentRecords{}).ByName(records) {
		presented = append(presented, route.present(entry))
	}
	return Answer{}.Ok(presented)
}

// Create stores a new signature, releasing its default addresses from the
// caller's other signatures.
func (route SignaturesRoute) Create(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Value(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxWrite(), "", "")
	if refusal != nil {
		return refusal
	}
	input, refusal := route.input(caller, body, false)
	if refusal != nil {
		return refusal
	}
	existing, err := route.Signatures.ListForOwner(caller.Email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	domain := caller.Domain
	if parts := strings.SplitN(caller.Email, "@", 2); len(parts) == 2 && parts[1] != "" {
		domain = parts[1]
	}
	now := EpochTime{}.Now()
	entry := map[string]any{
		"id":         ContentRecords{}.NewId("sig"),
		"owner":      caller.Email,
		"domain":     domain,
		"name":       ContentNames{}.Unique(TextValue{}.Of(input["name"]), existing, ""),
		"text":       RecordValues{}.Coalesce(input["text"], ""),
		"defaultFor": RecordValues{}.Coalesce(input["defaultFor"], []string{}),
		"createdAt":  now,
		"updatedAt":  now,
	}
	defaults, _ := entry["defaultFor"].([]string)
	if err := route.Signatures.ReleaseDefaults(caller.Email, defaults, entry["id"].(string)); err != nil {
		return RouteAccess{}.Failure(err)
	}
	if _, err := route.Signatures.Put(entry); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(route.present(entry))
}

// Update changes the fields the body names of one of the caller's
// signatures.
func (route SignaturesRoute) Update(id string, request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Value(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxWrite(), "", "")
	if refusal != nil {
		return refusal
	}
	documentId, existing, refusal := route.owned(caller, id)
	if refusal != nil {
		return refusal
	}
	patch, refusal := route.input(caller, body, true)
	if refusal != nil {
		return refusal
	}
	if refusal := route.content().Touched(caller, id, patch); refusal != nil {
		return refusal
	}
	if defaults, present := patch["defaultFor"].([]string); present {
		if err := route.Signatures.ReleaseDefaults(caller.Email, defaults, id); err != nil {
			return RouteAccess{}.Failure(err)
		}
	}
	return route.content().Saved(documentId, existing, patch, route.present)
}

// Delete deletes one of the caller's signatures.
func (route SignaturesRoute) Delete(id string, request *http.Request) *http.Response {
	return route.content().Remove(route.Access, request, id, map[string]any{"deleted": id})
}

func (route SignaturesRoute) input(caller RouteCaller, body any, partial bool) (map[string]any, *http.Response) {
	addresses, err := SenderAddresses{Users: route.Users}.Owned(caller.Email)
	if err != nil {
		return nil, RouteAccess{}.Failure(err)
	}
	input, problem := SignatureContent{}.Normalized(body, partial, addresses)
	if problem != "" {
		return nil, Answer{}.BadRequest(problem)
	}
	return input, nil
}

func (route SignaturesRoute) owned(caller RouteCaller, id string) (string, map[string]any, *http.Response) {
	if id == "" {
		return "", nil, Answer{}.NotFound("Signature not found")
	}
	return route.content().Lookup(caller, id)
}

func (route SignaturesRoute) content() OwnedContent {
	return OwnedContent{Find: route.Signatures.FindById, List: route.Signatures.ListForOwner, Update: route.Signatures.Update, Delete: route.Signatures.Delete, Missing: "Signature not found"}
}

func (SignaturesRoute) present(entry map[string]any) map[string]any {
	view := ContentRecords{}.Viewed(entry, "id", "domain", "name", "text")
	view["defaultFor"] = RecordValues{}.Coalesce(entry["defaultFor"], []any{})
	return view
}
