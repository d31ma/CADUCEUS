import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// InboxRuleValidation checks a normalized inbox rule: its name, matching,
// at most 20 conditions and between 1 and 10 actions.
type InboxRuleValidation struct{}

// Problem answers why the rule is refused, or "".
func (validation InboxRuleValidation) Problem(rule map[string]any) string {
	name, isText := rule["name"].(string)
	if !isText || (Security{}).Trim(name) == "" {
		return "name required"
	}
	if (TextValue{}).Length(Security{}.Trim(name)) > 200 || regexp.MustCompile(`[\x00-\x1f\x7f]`).MatchString(name) {
		return "Rule name is invalid"
	}
	if match := (RecordValues{}).Coalesce(rule["conditionMatch"], "all"); match != "all" && match != "any" {
		return "conditionMatch must be all or any"
	}
	for _, field := range []string{"enabled", "stopProcessing"} {
		if value, present := rule[field]; present {
			if _, isFlag := value.(bool); !isFlag {
				return field + " must be a boolean"
			}
		}
	}
	if order, present := rule["order"]; present {
		number, isNumber := RecordValues{}.Number(order)
		if !isNumber || number != math.Trunc(number) || number < 0 || number > 10_000 {
			return "order must be an integer from 0 through 10000"
		}
	}
	if problem := validation.conditions(RecordValues{}.Coalesce(rule["conditions"], []any{})); problem != "" {
		return problem
	}
	return validation.actions(RecordValues{}.Coalesce(rule["actions"], []any{}))
}

func (InboxRuleValidation) conditions(value any) string {
	conditions, isList := RecordValues{}.List(value)
	if !isList {
		return "Conditions must be an array"
	}
	if len(conditions) > 20 {
		return "At most 20 conditions are allowed"
	}
	fields := []string{"from", "to", "cc", "subject", "body", "hasAttachment"}
	operators := []string{"contains", "notContains", "equals", "notEquals", "startsWith", "endsWith"}
	for _, item := range conditions {
		condition, isObject := item.(map[string]any)
		field := TextValue{}.Of(RecordValues{}.Coalesce(condition["field"], "undefined"))
		if !isObject || !(PreferenceChoices{}).Includes(fields, field) {
			return "Unknown condition field: " + TextValue{}.Of(condition["field"])
		}
		operator := TextValue{}.Of(RecordValues{}.Coalesce(condition["op"], "undefined"))
		if !(PreferenceChoices{}).Includes(operators, operator) {
			return "Unknown condition operator: " + TextValue{}.Of(condition["op"])
		}
		value := TextValue{}.Of(RecordValues{}.Coalesce(condition["value"], "undefined"))
		if field == "hasAttachment" {
			if (operator != "equals" && operator != "notEquals") || (strings.ToLower(value) != "true" && strings.ToLower(value) != "false") {
				return "hasAttachment requires equals/notEquals and true/false"
			}
		} else if text, isText := condition["value"].(string); !isText || (Security{}).Trim(text) == "" {
			return "Every condition requires a non-empty value"
		}
		if (TextValue{}).Length(value) > 500 {
			return "Condition value is too long"
		}
	}
	return ""
}

func (InboxRuleValidation) actions(value any) string {
	actions, isList := RecordValues{}.List(value)
	if !isList || len(actions) == 0 {
		return "At least one action is required"
	}
	if len(actions) > 10 {
		return "At most 10 actions are allowed"
	}
	for _, item := range actions {
		action, isObject := item.(map[string]any)
		if !isObject || !(RecordValues{}).Truthy(action["type"]) {
			return "Action type is required"
		}
		kind := TextValue{}.Of(action["type"])
		if kind == "folder" {
			folderText, isText := action["folder"].(string)
			if !isText || folderText == "" {
				return "Folder action requires a folder name"
			}
			folder := strings.ToLower(Security{}.Trim(folderText))
			switch {
			case folder == "":
				return "Folder action requires a folder name"
			case TextValue{}.Length(folder) > 60:
				return "Folder name is too long"
			case !regexp.MustCompile(`^[a-z0-9][a-z0-9/_-]*$`).MatchString(folder):
				return "Folder may contain lowercase letters, numbers, hyphens, underscores, and slashes"
			}
		}
		if kind == "forward" {
			to, isText := action["to"].(string)
			if !isText || to == "" {
				return "Forward action requires a to address"
			}
			if (Security{}).NormalizeEmailAddress(to) == "" {
				return "Forward action requires a valid email address"
			}
		}
		if _, isFlag := action["value"].(bool); (kind == "markRead" || kind == "star") && !isFlag {
			return fmt.Sprintf("%s action requires a boolean value", kind)
		}
		if !(PreferenceChoices{}).Includes([]string{"folder", "forward", "delete", "markRead", "star"}, kind) {
			return "Unknown action type: " + kind
		}
	}
	return ""
}
