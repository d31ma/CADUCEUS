import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

// MailActionRules validates reversible bulk mail actions and the journal
// entries that make them reversible: the public request contract, the
// stored before and after states, and the result each action answers.
type MailActionRules struct{}

// MutableFields are the email fields an action changes.
func (MailActionRules) MutableFields() []string {
	return []string{"folder", "labels", "read", "starred"}
}

// IncompleteStatuses are the journal states an interrupted action leaves.
func (MailActionRules) IncompleteStatuses() []string {
	return []string{"prepared", "undoing", "redoing"}
}

// TimeToLive answers how long an action can be undone, in milliseconds.
func (MailActionRules) TimeToLive() int64 { return 10 * 60 * 1000 }

func (MailActionRules) mostMessages() int { return 100 }

func (MailActionRules) longestId() int { return 500 }

func (MailActionRules) corrupt() error {
	return &MailActionError{Message: "Mail action record is corrupt", Status: 503}
}

func (MailActionRules) refused(message string) error {
	return &MailActionError{Message: message, Status: 400}
}

func (MailActionRules) sortedKeys(fields map[string]any) []string {
	keys := []string{}
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Input answers the validated public bulk-action body, or why it is
// invalid.
func (rules MailActionRules) Input(input any) (MailActionInput, error) {
	body, isObject := input.(map[string]any)
	if !isObject {
		return MailActionInput{}, rules.refused("Action payload required")
	}
	list, isList := body["ids"].([]any)
	if !isList || len(list) == 0 {
		return MailActionInput{}, rules.refused("ids must be a non-empty array")
	}
	if len(list) > rules.mostMessages() {
		return MailActionInput{}, rules.refused(fmt.Sprintf("ids cannot contain more than %d messages", rules.mostMessages()))
	}
	ids := []string{}
	for _, value := range list {
		text, _ := value.(string)
		id := TextValue{}.Trim(text)
		if id == "" || (TextValue{}).Length(id) > rules.longestId() {
			return MailActionInput{}, rules.refused(fmt.Sprintf("every id must be between 1 and %d characters", rules.longestId()))
		}
		ids = append(ids, id)
	}
	if len(MailAddresses{}.Unique(ids)) != len(ids) {
		return MailActionInput{}, rules.refused("ids must not contain duplicates")
	}
	parsed := MailActionInput{Ids: ids}
	if value, given := body["clientRequestId"]; given {
		text, isText := value.(string)
		if !isText || !regexp.MustCompile(`^[A-Fa-f0-9-]{36}$`).MatchString(text) {
			return MailActionInput{}, rules.refused("clientRequestId must be a UUID")
		}
		parsed.ClientRequestId = strings.ToLower(text)
	}
	patch, err := rules.Patch(body["patch"])
	parsed.Patch = patch
	return parsed, err
}

// Patch answers a validated action patch, or why it is invalid.
func (rules MailActionRules) Patch(input any) (map[string]any, error) {
	source, isObject := input.(map[string]any)
	if !isObject {
		return nil, rules.refused("patch is required")
	}
	for _, key := range rules.sortedKeys(source) {
		if !(TextValue{}).Includes(rules.MutableFields(), key) {
			return nil, rules.refused("Unsupported patch field: " + key)
		}
	}
	patch := map[string]any{}
	if value, given := source["folder"]; given {
		text, isText := value.(string)
		if !isText || (TextValue{}).Trim(text) == "" {
			return nil, rules.refused("folder must be a non-empty string")
		}
		folder := CategoryPath{}.Normalized(text)
		if folder == "" {
			return nil, rules.refused("folder must be a valid category path")
		}
		patch["folder"] = folder
	}
	if value, given := source["labels"]; given {
		operation, err := MailActionLabels{}.Operation(value)
		if err != nil {
			return nil, err
		}
		patch["labels"] = operation
	}
	for _, field := range []string{"read", "starred"} {
		if value, given := source[field]; given {
			flag, isFlag := value.(bool)
			if !isFlag {
				return nil, rules.refused(field + " must be a boolean")
			}
			patch[field] = flag
		}
	}
	if len(patch) == 0 {
		return nil, rules.refused("No supported fields to update")
	}
	return patch, nil
}

// Changes answers the validated changes a journal entry stores.
func (rules MailActionRules) Changes(record map[string]any) ([]map[string]any, error) {
	text, _ := record["changesJson"].(string)
	var value any
	if json.Unmarshal([]byte(text), &value) != nil {
		return nil, rules.corrupt()
	}
	list, isList := value.([]any)
	if !isList || len(list) == 0 || len(list) > rules.mostMessages() {
		return nil, rules.corrupt()
	}
	changes := []map[string]any{}
	for _, item := range list {
		change, isObject := item.(map[string]any)
		_, hasId := change["id"].(string)
		_, hasDomain := change["domain"].(string)
		if !isObject || !hasId || !hasDomain || !rules.validChange(change) {
			return nil, rules.corrupt()
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func (rules MailActionRules) validChange(change map[string]any) bool {
	before, after := rules.validState(change["before"]), rules.validState(change["after"])
	if before == nil || after == nil || strings.Join(rules.sortedKeys(before), ",") != strings.Join(rules.sortedKeys(after), ",") {
		return false
	}
	source, given := change["source"]
	if !given {
		return true
	}
	if source != "provider" {
		return false
	}
	stableId, isText := change["stableId"].(string)
	if !(ExternalMessage{}).IsAccountId(change["accountId"]) || !isText || len(stableId) < 1 || (TextValue{}).Length(stableId) > 512 || strings.ContainsAny(stableId, "\x00\r\n") {
		return false
	}
	mutations, _ := change["providerMutations"].(map[string]any)
	generations, _ := change["providerGenerations"].(map[string]any)
	for _, target := range []string{"before", "after"} {
		ids, isList := mutations[target].([]any)
		if !isList || len(ids) > len(rules.MutableFields()) {
			return false
		}
		for _, id := range ids {
			text, isText := id.(string)
			if !isText || !regexp.MustCompile(`^mut_[A-Za-z0-9_-]{20,80}$`).MatchString(text) {
				return false
			}
		}
		generation, isNumber := generations[target].(float64)
		if !isNumber || generation != math.Trunc(generation) || generation < 0 || generation > 10_000 {
			return false
		}
	}
	return true
}

// validState answers a stored before or after state, or nil when it is not
// a non-empty object of valid mutable fields.
func (rules MailActionRules) validState(value any) map[string]any {
	state, isObject := value.(map[string]any)
	if !isObject || len(state) == 0 {
		return nil
	}
	for field, item := range state {
		if !(TextValue{}).Includes(rules.MutableFields(), field) {
			return nil
		}
		switch field {
		case "folder":
			if text, isText := item.(string); !isText || text == "" {
				return nil
			}
		case "labels":
			list, isList := item.([]any)
			if !isList || len(list) > 100 || len(MailActionLabels{}.Stored(list)) != len(list) {
				return nil
			}
		default:
			if _, isFlag := item.(bool); !isFlag {
				return nil
			}
		}
	}
	return state
}

// EnsureDomains refuses changes to a hosted domain outside the caller's.
func (MailActionRules) EnsureDomains(changes []map[string]any, domains []string) error {
	for _, change := range changes {
		if change["source"] != "provider" && !(TextValue{}).Includes(domains, change["domain"]) {
			return &MailActionError{Message: "Mail action not found", Status: 404}
		}
	}
	return nil
}

// FieldValue answers one mutable field of an email as an action compares
// it.
func (MailActionRules) FieldValue(email map[string]any, field string) any {
	switch field {
	case "folder":
		return TextValue{}.Or(email["folder"], "inbox")
	case "labels":
		return MailActionLabels{}.Stored(email["labels"])
	}
	return email[field] == true
}

// MatchesState reports whether an email holds every field of a state.
func (rules MailActionRules) MatchesState(email map[string]any, state map[string]any) bool {
	for field, value := range state {
		if !rules.Equal(rules.FieldValue(email, field), value) {
			return false
		}
	}
	return true
}

// SameState reports whether two states hold the same fields and values.
func (rules MailActionRules) SameState(left map[string]any, right map[string]any) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if !rules.Equal(value, right[key]) {
			return false
		}
	}
	return true
}

// Equal compares two field values: lists item by item, anything else as
// the same text, flag or nothing.
func (MailActionRules) Equal(left any, right any) bool {
	leftList, leftIsList := RecordValues{}.List(left)
	rightList, rightIsList := RecordValues{}.List(right)
	if leftIsList || rightIsList {
		if !leftIsList || !rightIsList || len(leftList) != len(rightList) {
			return false
		}
		for index := range leftList {
			if leftList[index] != rightList[index] {
				return false
			}
		}
		return true
	}
	switch left.(type) {
	case string, bool, float64, nil:
		return left == right
	}
	return false
}

// Kind answers the kind of action a patch makes on count messages.
func (MailActionRules) Kind(patch map[string]any, count int) string {
	folder, hasFolder := patch["folder"].(string)
	switch {
	case count > 1 || len(patch) > 1:
		return "bulk"
	case folder == "archive" || folder == "trash":
		return folder
	case hasFolder:
		return "move"
	case patch["read"] == true:
		return "mark-read"
	case patch["read"] == false:
		return "mark-unread"
	case patch["starred"] == true:
		return "star"
	case patch["labels"] != nil:
		return "label"
	}
	return "unstar"
}

// Result answers an action's result with the state of each email.
func (MailActionRules) Result(actionId string, record map[string]any, status string, emails []map[string]any) map[string]any {
	text := TextValue{}
	presented := []map[string]any{}
	for _, email := range emails {
		item := map[string]any{
			"id": email["id"], "domain": email["domain"], "folder": text.Or(email["folder"], "inbox"),
			"labels": MailActionLabels{}.Stored(email["labels"]), "read": email["read"] == true, "starred": email["starred"] == true,
		}
		if text.Truthy(email["external"]) {
			item["external"] = true
			item["providerAccountId"] = email["providerAccountId"]
		}
		presented = append(presented, item)
	}
	return map[string]any{
		"actionId": actionId, "status": status, "kind": record["kind"], "count": len(emails),
		"expiresAt": record["expiresAt"], "emails": presented,
	}
}

// Fingerprint answers the SHA-256 of the ids and the key-sorted patch, as
// JSON, which an idempotent replay must match.
func (MailActionRules) Fingerprint(ids []string, patch map[string]any) string {
	return RandomText{}.Digest([]byte(TextValue{}.Json(map[string]any{"ids": ids, "patch": patch})))
}

// Decoded answers changes as JSON decodes them, so built and stored changes
// carry the same value types.
func (MailActionRules) Decoded(changes []map[string]any) []map[string]any {
	decoded := []map[string]any{}
	_ = json.Unmarshal([]byte(TextValue{}.Json(changes)), &decoded)
	return decoded
}
