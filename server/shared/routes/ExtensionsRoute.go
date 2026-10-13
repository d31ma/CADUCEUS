import (
	"net/http"
	"sort"
)

// ExtensionsRoute answers /extensions, /extensions/inspect and
// /extensions/{id}: the trusted extensions of the caller's own mailbox.
type ExtensionsRoute struct {
	Access     *AccessHelper
	Extensions *ExtensionsHelper
}

// List answers the caller's extensions by display name, without source.
func (route ExtensionsRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxRead(), "", "")
	if refusal != nil {
		return refusal
	}
	records, err := route.Extensions.ListForOwner(caller.Email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	sort.SliceStable(records, func(left, right int) bool {
		return TextOrder{}.Compare(RecordValues{}.Text(records[left]["displayName"]), RecordValues{}.Text(records[right]["displayName"])) < 0
	})
	presented := []any{}
	for _, record := range records {
		view, err := Extensions{}.Present(record, false)
		if err != nil {
			return RouteAccess{}.Failure(err)
		}
		presented = append(presented, view)
	}
	return Answer{}.Ok(presented)
}

// Install stores a package whose inspected digest the caller trusted.
func (route ExtensionsRoute) Install(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxWrite(), "", "")
	if refusal != nil {
		return refusal
	}
	inspected, problem := Extensions{}.Inspect(body["bundle"])
	if problem != "" {
		return Answer{}.BadRequest(problem)
	}
	if trusted, isText := body["trustedDigest"].(string); !isText || trusted != inspected["digest"] {
		return Answer{}.BadRequest("The inspected extension digest must be explicitly trusted")
	}
	manifest, _ := inspected["manifest"].(map[string]any)
	existing, err := route.Extensions.FindForOwner(caller.Email, RecordValues{}.Text(manifest["id"]))
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if existing != nil {
		return Answer{}.Conflict("Extension is already installed")
	}
	record := Extensions{}.TrustedRecord(caller.Email, inspected)
	if _, err := route.Extensions.Put(record); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return route.present(record, false)
}

// Inspect answers what a package would install, without storing it.
func (route ExtensionsRoute) Inspect(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	if _, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxRead(), "", ""); refusal != nil {
		return refusal
	}
	inspected, problem := Extensions{}.Inspect(body["bundle"])
	if problem != "" {
		return Answer{}.BadRequest(problem)
	}
	inspected["enabled"] = false
	return route.present(inspected, false)
}

// Read answers one owned extension with its source, for the runtime.
func (route ExtensionsRoute) Read(id string, request *http.Request) *http.Response {
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxRead(), "", "")
	if refusal != nil {
		return refusal
	}
	record, refusal := route.owned(caller.Email, id)
	if refusal != nil {
		return refusal
	}
	return route.present(record, true)
}

// Enable switches one owned extension on or off; enabling a theme switches
// the owner's other themes off.
func (route ExtensionsRoute) Enable(id string, request *http.Request) *http.Response {
	value, refusal := RequestBody{}.Value(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxWrite(), "", "")
	if refusal != nil {
		return refusal
	}
	body, isObject := value.(map[string]any)
	enabled, isFlag := body["enabled"].(bool)
	if !isObject || len(body) != 1 || !isFlag {
		return Answer{}.BadRequest("enabled must be the only field")
	}
	record, refusal := route.owned(caller.Email, id)
	if refusal != nil {
		return refusal
	}
	now := EpochTime{}.NowText()
	if enabled && record["kind"] == "theme" {
		if err := route.disableOtherThemes(caller.Email, record["docId"], now); err != nil {
			return RouteAccess{}.Failure(err)
		}
	}
	if err := route.Extensions.Update(RecordValues{}.Text(record["docId"]), map[string]any{"enabled": enabled, "updatedAt": now}); err != nil {
		return RouteAccess{}.Failure(err)
	}
	record["enabled"] = enabled
	record["updatedAt"] = now
	return route.present(record, false)
}

// Remove deletes one owned extension.
func (route ExtensionsRoute) Remove(id string, request *http.Request) *http.Response {
	if _, refusal := (RequestBody{}).Value(request); refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxWrite(), "", "")
	if refusal != nil {
		return refusal
	}
	record, refusal := route.owned(caller.Email, id)
	if refusal != nil {
		return refusal
	}
	if err := route.Extensions.Delete(RecordValues{}.Text(record["docId"])); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(map[string]any{"deleted": true})
}

func (route ExtensionsRoute) disableOtherThemes(owner string, documentId any, now string) error {
	others, err := route.Extensions.ListForOwner(owner)
	if err != nil {
		return err
	}
	for _, other := range others {
		if enabled, _ := other["enabled"].(bool); other["kind"] == "theme" && enabled && other["docId"] != documentId {
			if err := route.Extensions.Update(RecordValues{}.Text(other["docId"]), map[string]any{"enabled": false, "updatedAt": now}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (route ExtensionsRoute) owned(owner string, id string) (map[string]any, *http.Response) {
	if id == "" {
		return nil, Answer{}.NotFound("Extension not found")
	}
	record, err := route.Extensions.FindForOwner(owner, id)
	if err != nil {
		return nil, RouteAccess{}.Failure(err)
	}
	if record == nil {
		return nil, Answer{}.NotFound("Extension not found")
	}
	return record, nil
}

func (ExtensionsRoute) present(record map[string]any, runtime bool) *http.Response {
	view, err := Extensions{}.Present(record, runtime)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(view)
}
