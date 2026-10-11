import (
	"cmp"
	"regexp"
	"strings"
)

// RuleDescriptionUtils writes the human-readable summaries of inbox rules
// and normalizes the folder names a rule may move mail into.
type RuleDescriptionUtils struct{}

func (utils RuleDescriptionUtils) Conditions(rule InboxRule) string {
	return cmp.Or(utils.conditions(rule.Conditions, utils.spaced(rule.ConditionMatch)), "Always")
}

func (utils RuleDescriptionUtils) Condition(condition RuleCondition) string {
	return condition.Field + utils.spaced(condition.Op) + `"` + condition.Value + `"`
}

func (utils RuleDescriptionUtils) Actions(actions []RuleAction) string {
	return utils.actions(actions)
}

func (utils RuleDescriptionUtils) Action(action RuleAction) string {
	return utils.describe(action, utils.flag(action.Value))
}

// FolderName keeps a plug-and-play folder path: lower case, dashes for
// spaces, only path-safe characters and at most sixty of them.
func (RuleDescriptionUtils) FolderName(value string) string {
	text := strings.ToLower(strings.TrimSpace(value))
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, "-")
	text = regexp.MustCompile(`[^a-z0-9/_-]`).ReplaceAllString(text, "")
	separator := "/"
	text = regexp.MustCompile(`/+`).ReplaceAllString(text, separator)
	text = strings.Trim(text, separator)
	return text[:min(60, len(text))]
}

func (utils RuleDescriptionUtils) conditions(conditions []RuleCondition, joiner string) string {
	labels := []string{}
	for _, condition := range conditions {
		labels = append(labels, utils.Condition(condition))
	}
	return strings.Join(labels, joiner)
}

func (utils RuleDescriptionUtils) actions(actions []RuleAction) string {
	labels := []string{}
	for _, action := range actions {
		labels = append(labels, utils.Action(action))
	}
	return strings.Join(labels, ", ")
}

func (RuleDescriptionUtils) spaced(word string) string {
	return strings.Join([]string{"", word, ""}, " ")
}

func (RuleDescriptionUtils) flag(value *bool) bool {
	return value != nil && *value
}

func (RuleDescriptionUtils) describe(action RuleAction, value bool) string {
	switch action.Type {
	case "folder":
		return `→ folder "` + action.Folder + `"`
	case "forward":
		return "→ forward to " + action.To
	case "markRead":
		return "→ mark " + ChoiceUtils{}.Text(value, "read", "unread")
	case "star":
		return "→ " + ChoiceUtils{}.Text(value, action.Type, "un"+action.Type)
	}
	return "→ delete"
}
