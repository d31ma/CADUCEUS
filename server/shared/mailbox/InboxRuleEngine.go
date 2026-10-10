import "strings"

// InboxRuleEngine evaluates one normalized inbox rule against a message
// without side effects; delivery and the preview route share it.
type InboxRuleEngine struct{}

// Evaluated answers whether the rule matched, how each condition fared, the
// actions it would take and whether it stops later rules.
func (engine InboxRuleEngine) Evaluated(rule map[string]any, input map[string]any) map[string]any {
	message := engine.message(input)
	conditions, _ := RecordValues{}.List(RecordValues{}.Coalesce(rule["conditions"], []any{}))
	results := []any{}
	all, some := true, false
	for _, item := range conditions {
		condition, _ := item.(map[string]any)
		matched := engine.matches(condition, message)
		result := TextValue{}.Copy(condition)
		result["matched"] = matched
		results = append(results, result)
		all, some = all && matched, some || matched
	}
	matched := len(results) == 0 || (rule["conditionMatch"] == "all" && all) || (rule["conditionMatch"] != "all" && some)
	planned := []any{}
	if matched {
		actions, _ := RecordValues{}.List(RecordValues{}.Coalesce(rule["actions"], []any{}))
		for _, action := range actions {
			object, _ := action.(map[string]any)
			planned = append(planned, TextValue{}.Copy(object))
		}
	}
	return map[string]any{
		"matched":          matched,
		"conditionResults": results,
		"plannedActions":   planned,
		"stopProcessing":   matched && (RecordValues{}).Truthy(rule["stopProcessing"]),
	}
}

func (engine InboxRuleEngine) message(input map[string]any) map[string]any {
	return map[string]any{
		"from":          engine.text(RecordValues{}.Coalesce(input["from"], input["sender"])),
		"to":            engine.text(RecordValues{}.Coalesce(input["to"], input["recipient"])),
		"cc":            engine.text(input["cc"]),
		"subject":       engine.text(input["subject"]),
		"body":          engine.text(input["body"]),
		"hasAttachment": RecordValues{}.Truthy(input["hasAttachment"]),
	}
}

func (InboxRuleEngine) matches(condition map[string]any, message map[string]any) bool {
	field := condition["field"]
	if field == "hasAttachment" {
		expected := strings.ToLower(TextValue{}.Of(RecordValues{}.Coalesce(condition["value"], "undefined"))) == "true"
		if condition["op"] == "notEquals" {
			return message["hasAttachment"] != expected
		}
		return message["hasAttachment"] == expected
	}
	if !(TextValue{}).Includes([]string{"from", "to", "cc", "subject", "body"}, field) {
		return false
	}
	value := strings.ToLower(message[field.(string)].(string))
	needle := strings.ToLower(TextValue{}.Of(condition["value"]))
	switch condition["op"] {
	case "contains":
		return strings.Contains(value, needle)
	case "notContains":
		return !strings.Contains(value, needle)
	case "equals":
		return value == needle
	case "notEquals":
		return value != needle
	case "startsWith":
		return strings.HasPrefix(value, needle)
	case "endsWith":
		return strings.HasSuffix(value, needle)
	}
	return false
}

// text answers String(value) for a message field, with lists joined by
// ", " and null or undefined as "".
func (InboxRuleEngine) text(value any) string {
	if list, isList := (RecordValues{}).List(value); isList {
		parts := []string{}
		for _, item := range list {
			parts = append(parts, TextValue{}.Of(item))
		}
		return strings.Join(parts, ", ")
	}
	return TextValue{}.Of(value)
}
