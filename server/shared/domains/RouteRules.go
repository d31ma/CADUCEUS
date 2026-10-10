import (
	"fmt"
	"strings"
)

// RouteRules validates the inbound routing rules of a domain. A rule is
// decoded JSON, so every field is checked for its kind as well as its value.
type RouteRules struct{}

// Invalid answers why rules are not valid, or "" when they are.
func (rules RouteRules) Invalid(value any) string {
	list, isList := value.([]any)
	if !isList {
		return "Routes must be an array"
	}
	seen := map[string]bool{}
	for _, item := range list {
		rule, _ := item.(map[string]any)
		id, _ := rule["id"].(string)
		if id == "" {
			return "Each route must have an id"
		}
		if seen[id] {
			return "Duplicate route id: " + id
		}
		seen[id] = true
		if match, _ := rule["match"].(string); strings.TrimSpace(match) == "" {
			return fmt.Sprintf("Route %s: match is required", id)
		}
		if problem := rules.InvalidAction(rule["action"]); problem != "" {
			return fmt.Sprintf("Route %s: %s", id, problem)
		}
	}
	return ""
}

// InvalidAction answers why one route action is not valid, or "".
func (RouteRules) InvalidAction(value any) string {
	action, isObject := value.(map[string]any)
	if !isObject || action == nil {
		return "Action is required"
	}
	switch action["type"] {
	case "webhook":
		target, _ := action["url"].(string)
		if target == "" {
			return "Webhook URL is required"
		}
		return Security{}.SafeWebhookUrl(target)
	case "forward":
		if to, _ := action["to"].(string); to == "" {
			return "Forward target is required"
		}
		return ""
	case "store", "drop":
		return ""
	default:
		return "Unknown action type"
	}
}
