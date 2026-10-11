import "net/http"

// DraftsRoute answers /drafts and /drafts/{id}: the caller's durable drafts,
// private to the mailbox that owns them.
type DraftsRoute struct {
	Access *AccessHelper
	Drafts *DraftService
	Users  *UsersHelper
}

// Create answers POST /drafts (idempotent per clientId).
func (route DraftsRoute) Create(request *http.Request) *http.Response {
	caller, body, refusal := (DeliveryReply{}).Caller(route.Access, request)
	if refusal != nil {
		return refusal
	}
	return DeliveryReply{}.Settled(route.create(caller, body))
}

func (route DraftsRoute) create(caller RouteCaller, body map[string]any) (any, error) {
	input, answer, err := route.input(caller, body)
	if answer != nil || err != nil {
		return answer, err
	}
	draft, recovered, err := route.Drafts.Drafts().Create(caller.Email, input)
	if err != nil {
		return nil, err
	}
	return DeliveryFields{}.Merged(draft, map[string]any{"recovered": recovered}), nil
}

// input parses a snapshot and resolves its owned From address.
func (route DraftsRoute) input(caller RouteCaller, body map[string]any) (map[string]any, any, error) {
	input, problem := DraftInput{}.Parsed(body)
	if problem != "" {
		return nil, map[string]any{"error": problem}, nil
	}
	sender, err := SenderAddresses{Users: route.Users}.Sender(caller.Email, input["from"])
	if err != nil {
		return nil, nil, err
	}
	if sender == "" {
		return nil, map[string]any{"error": "From address is not owned by this mailbox", "forbidden": true}, nil
	}
	input["from"] = sender
	return input, nil, nil
}

// Get answers GET /drafts/{id}.
func (route DraftsRoute) Get(id string, request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	draft, err := route.Drafts.Drafts().Get(id, caller.Email, caller.Domains)
	if err == nil && (id == "" || draft == nil) {
		return Answer{}.NotFound("Draft not found")
	}
	return DeliveryReply{}.Settled(draft, err)
}

// Update answers PUT /drafts/{id}: compare-and-swap on the version.
func (route DraftsRoute) Update(id string, request *http.Request) *http.Response {
	caller, body, refusal := (DeliveryReply{}).Caller(route.Access, request)
	if refusal != nil {
		return refusal
	}
	return DeliveryReply{}.Settled(route.update(caller, id, body))
}

func (route DraftsRoute) update(caller RouteCaller, id string, body map[string]any) (any, error) {
	if id == "" {
		return map[string]any{"error": "Draft not found", "notFound": true}, nil
	}
	version, isNumber := body["version"].(float64)
	if !isNumber || version != float64(int(version)) || version < 1 {
		return map[string]any{"error": "version is required"}, nil
	}
	input, answer, err := route.input(caller, body)
	if answer != nil || err != nil {
		return answer, err
	}
	kind, draft, err := route.Drafts.Drafts().Update(id, caller.Email, caller.Domains, int(version), input)
	switch {
	case err != nil:
		return nil, err
	case kind == "not-found":
		return map[string]any{"error": "Draft not found", "notFound": true}, nil
	case kind == "conflict":
		return DeliveryFields{}.Merged(draft, map[string]any{"error": "Draft changed in another session", "conflict": true}), nil
	}
	return draft, nil
}

// Discard answers DELETE /drafts/{id}.
func (route DraftsRoute) Discard(id string, request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	discarded, err := route.Drafts.Drafts().Discard(id, caller.Email, caller.Domains)
	if err == nil && (id == "" || !discarded) {
		return Answer{}.NotFound("Draft not found")
	}
	return DeliveryReply{}.Settled(map[string]any{"discarded": id}, err)
}
