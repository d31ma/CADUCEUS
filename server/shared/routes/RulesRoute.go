import (
	"net/http"
	"strings"
)

// RulesRoute answers /rules, /rules/{id}, /rules/order and /rules/preview:
// the inbox rules a mailbox owns.
type RulesRoute struct {
	Access *AccessHelper
	Rules  *RulesHelper
	Users  *UsersHelper
}

// List answers the caller's rules in order.
func (route RulesRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	return route.answered(route.Rules.ListForOwner(caller.Email))
}

func (RulesRoute) answered(rules []map[string]any, err error) *http.Response {
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(rules)
}

// Create stores a new rule for one of the caller's own mailboxes.
func (route RulesRoute) Create(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxWrite(), "", "")
	if refusal != nil {
		return refusal
	}
	mailbox, refusal := route.mailbox(caller, body["mailbox"])
	if refusal != nil {
		return refusal
	}
	order, err := route.Rules.NextOrder(caller.Email)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	ruleId := RandomText{}.Hex(16)
	if body == nil {
		body = map[string]any{}
	}
	rule := InboxRuleInput{}.Normalized(body, InboxRuleIdentity{Id: ruleId, OwnerEmail: caller.Email, Mailbox: mailbox, Order: order})
	if problem := route.problem(rule, mailbox, false); problem != "" {
		return Answer{}.BadRequest(problem)
	}
	if _, err := route.Rules.Put(rule); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(map[string]any{"id": ruleId})
}

// Delete deletes one of the caller's rules.
func (route RulesRoute) Delete(id string, request *http.Request) *http.Response {
	if _, refusal := (RequestBody{}).Value(request); refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxWrite(), "", "")
	if refusal != nil {
		return refusal
	}
	if id == "" {
		return Answer{}.BadRequest("rule id required")
	}
	documentId, _, refusal := route.owned(caller, id)
	if refusal != nil {
		return refusal
	}
	if err := route.Rules.Delete(documentId); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(map[string]any{"deleted": id})
}

// Update changes the editable fields of one of the caller's rules.
func (route RulesRoute) Update(id string, request *http.Request) *http.Response {
	value, refusal := RequestBody{}.Value(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxWrite(), "", "")
	if refusal != nil {
		return refusal
	}
	if id == "" || value == nil {
		return Answer{}.BadRequest("rule id and body required")
	}
	documentId, existing, refusal := route.owned(caller, id)
	if refusal != nil {
		return refusal
	}
	merged := TextValue{}.Copy(existing)
	body, _ := value.(map[string]any)
	for _, field := range route.editable() {
		if change, present := body[field]; present {
			merged[field] = change
		}
	}
	identity := InboxRuleIdentity{
		Id:         TextValue{}.Of(RecordValues{}.Or(existing["id"], "")),
		OwnerEmail: TextValue{}.Of(RecordValues{}.Coalesce(existing["ownerEmail"], caller.Email)),
		Mailbox:    TextValue{}.Of(RecordValues{}.Coalesce(existing["mailbox"], caller.Email)),
		Order:      RecordValues{}.Coalesce(existing["order"], 0.0),
	}
	rule := InboxRuleInput{}.Normalized(merged, identity)
	if problem := route.problem(rule, TextValue{}.Of(existing["mailbox"]), true); problem != "" {
		return Answer{}.BadRequest(problem)
	}
	patch := map[string]any{}
	for _, field := range route.editable() {
		if change, present := rule[field]; present {
			patch[field] = change
		}
	}
	if err := route.Rules.Update(documentId, patch); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(map[string]any{"updated": id})
}

// Reorder stores the caller's complete rule order.
func (route RulesRoute) Reorder(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Allowed(request, RouteAccess{}.MailboxWrite(), "", "")
	if refusal != nil {
		return refusal
	}
	list, isList := body["ids"].([]any)
	ids := []string{}
	for _, item := range list {
		id, isText := item.(string)
		if !isText || (PreferenceChoices{}).Includes(ids, id) {
			isList = false
			break
		}
		ids = append(ids, id)
	}
	if !isList || len(list) > 100 {
		return Answer{}.BadRequest("ids must be a unique list of rule IDs")
	}
	ordered, err := route.Rules.Reorder(caller.Email, ids)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if !ordered {
		return Answer{}.BadRequest("ids must include every owned rule exactly once")
	}
	return Answer{}.Ok(map[string]any{"ordered": len(ids)})
}

// Preview answers what a stored or unsaved rule would do to a message.
func (route RulesRoute) Preview(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	message, isObject := body["message"].(map[string]any)
	if !isObject || message == nil {
		return Answer{}.BadRequest("message required")
	}
	if problem := route.messageProblem(message); problem != "" {
		return Answer{}.BadRequest(problem)
	}
	rule, refusal := route.previewed(caller, body)
	if refusal != nil {
		return refusal
	}
	return Answer{}.Ok(InboxRuleEngine{}.Evaluated(rule, message))
}

func (route RulesRoute) previewed(caller RouteCaller, body map[string]any) (map[string]any, *http.Response) {
	if (RecordValues{}).Truthy(body["ruleId"]) {
		_, existing, err := route.Rules.FindById(TextValue{}.Of(body["ruleId"]))
		if err != nil {
			return nil, RouteAccess{}.Failure(err)
		}
		if existing == nil || existing["ownerEmail"] != strings.ToLower(caller.Email) {
			return nil, Answer{}.NotFound("Rule not found")
		}
		return existing, nil
	}
	input, _ := body["rule"].(map[string]any)
	mailbox, refusal := route.mailbox(caller, input["mailbox"])
	if refusal != nil {
		return nil, refusal
	}
	if input == nil {
		input = map[string]any{}
	}
	rule := InboxRuleInput{}.Normalized(input, InboxRuleIdentity{OwnerEmail: caller.Email, Mailbox: mailbox, Order: 0.0})
	if problem := (InboxRuleValidation{}).Problem(rule); problem != "" {
		return nil, Answer{}.BadRequest(problem)
	}
	return rule, nil
}

// mailbox answers the requested mailbox (the caller's own address when none
// is named) when it is one of the caller's sender addresses.
func (route RulesRoute) mailbox(caller RouteCaller, requested any) (string, *http.Response) {
	owned, err := SenderAddresses{Users: route.Users}.Owned(caller.Email)
	if err != nil {
		return "", RouteAccess{}.Failure(err)
	}
	mailbox := caller.Email
	if text, isText := requested.(string); isText {
		mailbox = strings.ToLower(Security{}.Trim(text))
	}
	if !(PreferenceChoices{}).Includes(owned, mailbox) {
		return "", Answer{}.Forbidden("Mailbox access denied")
	}
	return mailbox, nil
}

func (route RulesRoute) owned(caller RouteCaller, id string) (string, map[string]any, *http.Response) {
	documentId, existing, err := route.Rules.FindById(id)
	if err != nil {
		return "", nil, RouteAccess{}.Failure(err)
	}
	if documentId == "" || existing == nil || existing["ownerEmail"] != strings.ToLower(caller.Email) {
		return "", nil, Answer{}.NotFound("Rule not found")
	}
	return documentId, existing, nil
}

// problem answers why a rule is refused, including a forward back to its own
// mailbox (normalizing the target first when trim is set).
func (RulesRoute) problem(rule map[string]any, mailbox string, trim bool) string {
	if problem := (InboxRuleValidation{}).Problem(rule); problem != "" {
		return problem
	}
	actions, _ := RecordValues{}.List(rule["actions"])
	for _, item := range actions {
		action, _ := item.(map[string]any)
		target := TextValue{}.Of(action["to"])
		if trim {
			target = strings.ToLower(Security{}.Trim(target))
		}
		if action["type"] == "forward" && target == mailbox {
			return "A rule cannot forward a message back to its own mailbox"
		}
	}
	return ""
}

func (RulesRoute) messageProblem(message map[string]any) string {
	for _, field := range []string{"from", "to", "cc", "subject"} {
		value, present := message[field]
		_, isText := value.(string)
		_, isList := value.([]any)
		if present && !isText && !isList {
			return field + " must be text or a list"
		}
		if (TextValue{}).Length(TextValue{}.Of(value)) > 5_000 {
			return field + " is too long"
		}
	}
	if value, present := message["body"]; present {
		if text, isText := value.(string); !isText || (TextValue{}).Length(text) > 100_000 {
			return "body is too long"
		}
	}
	if value, present := message["hasAttachment"]; present {
		if _, isFlag := value.(bool); !isFlag {
			return "hasAttachment must be a boolean"
		}
	}
	return ""
}

func (RulesRoute) editable() []string {
	return []string{"name", "enabled", "conditionMatch", "conditions", "actions", "stopProcessing"}
}
