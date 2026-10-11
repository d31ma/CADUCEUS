import (
	"errors"
	"net/http"
)

// MailActionsRoute answers /inbox/actions and its undo and redo: durable,
// reversible bulk mutations of the caller's mail.
type MailActionsRoute struct {
	Access  *AccessHelper
	Actions *MailActionService
}

// Apply validates and applies one bulk action.
func (route MailActionsRoute) Apply(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	body, refusal := RequestBody{}.Value(request)
	if refusal != nil {
		return refusal
	}
	input, err := MailActionRules{}.Input(body)
	if err != nil {
		return route.failure(err)
	}
	result, err := route.Actions.Apply(caller, input, EpochTime{}.Now())
	return route.presented(caller, result, err)
}

// Undo restores what one action replaced.
func (route MailActionsRoute) Undo(id string, request *http.Request) *http.Response {
	return route.transition(id, request, route.Actions.Undo)
}

// Redo applies one undone action again.
func (route MailActionsRoute) Redo(id string, request *http.Request) *http.Response {
	return route.transition(id, request, route.Actions.Redo)
}

func (route MailActionsRoute) transition(id string, request *http.Request, change func(RouteCaller, string, int64) (map[string]any, error)) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	if id == "" {
		return Answer{}.NotFound("Mail action not found")
	}
	result, err := change(caller, id, EpochTime{}.Now())
	return route.presented(caller, result, err)
}

// presented answers an action result with its expiry as ISO text and each
// email with its category role.
func (route MailActionsRoute) presented(caller RouteCaller, result map[string]any, err error) *http.Response {
	if err != nil {
		return route.failure(err)
	}
	mappings, err := route.Actions.Mappings(caller.Email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	presented := TextValue{}.Copy(result)
	presented["expiresAt"] = EpochTime{}.Text(int64(RecordValues{}.NumberOf(result["expiresAt"])))
	emails, _ := result["emails"].([]map[string]any)
	presented["emails"] = mappings.PresentEmails(emails)
	return Answer{}.Ok(presented)
}

func (MailActionsRoute) failure(err error) *http.Response {
	var refusal *MailActionError
	if !errors.As(err, &refusal) {
		return RouteAccess{}.Failure(err)
	}
	switch refusal.Status {
	case http.StatusNotFound:
		return Answer{}.NotFound(refusal.Message)
	case http.StatusConflict:
		return Answer{}.Conflict(refusal.Message)
	case http.StatusServiceUnavailable:
		return Answer{}.Unavailable(refusal.Message)
	}
	return Answer{}.BadRequest(refusal.Message)
}
