import "strings"

// InboxRuleInput normalizes the user-controlled parts of a mailbox rule
// before validation and storage. A field the input leaves out stays out, as
// undefined did.
type InboxRuleInput struct{}

// Normalized answers the rule of input for an identity.
func (rules InboxRuleInput) Normalized(input map[string]any, identity InboxRuleIdentity) map[string]any {
	ownerEmail := Security{}.NormalizeEmailAddress(identity.OwnerEmail)
	mailbox := Security{}.NormalizeEmailAddress(identity.Mailbox)
	domain := ""
	if parts := strings.SplitN(mailbox, "@", 2); len(parts) == 2 {
		domain = parts[1]
	}
	name, _ := input["name"].(string)
	rule := map[string]any{
		"ownerEmail":     ownerEmail,
		"mailbox":        mailbox,
		"domain":         domain,
		"name":           Security{}.Trim(name),
		"enabled":        true,
		"order":          identity.Order,
		"stopProcessing": input["stopProcessing"] == true,
		"conditionMatch": RecordValues{}.Coalesce(input["conditionMatch"], "all"),
	}
	if identity.Id != "" {
		rule["id"] = identity.Id
	}
	if enabled, present := input["enabled"]; present {
		rule["enabled"] = enabled
	}
	for field, normalize := range map[string]func(any) any{"conditions": rules.condition, "actions": rules.action} {
		value, present := input[field]
		list, isList := RecordValues{}.List(value)
		switch {
		case isList:
			normalized := []any{}
			for _, item := range list {
				normalized = append(normalized, normalize(item))
			}
			rule[field] = normalized
		case present:
			rule[field] = value
		}
	}
	return rule
}

func (InboxRuleInput) condition(value any) any {
	condition, _ := value.(map[string]any)
	field := TextValue{}.Of(condition["field"])
	normalized := map[string]any{"field": field, "op": TextValue{}.Of(condition["op"])}
	raw, present := condition["value"]
	switch text, isText := raw.(string); {
	case field == "hasAttachment":
		normalized["value"] = strings.ToLower(Security{}.Trim(TextValue{}.Of(raw)))
	case isText:
		normalized["value"] = Security{}.Trim(text)
	case present:
		normalized["value"] = raw
	}
	return normalized
}

func (InboxRuleInput) action(value any) any {
	action, _ := value.(map[string]any)
	kind := action["type"]
	switch kind {
	case "folder":
		return map[string]any{"type": "folder", "folder": strings.ToLower(Security{}.Trim(TextValue{}.Of(action["folder"])))}
	case "forward":
		to := Security{}.NormalizeEmailAddress(TextValue{}.Of(action["to"]))
		if to == "" {
			to = strings.ToLower(Security{}.Trim(TextValue{}.Of(action["to"])))
		}
		return map[string]any{"type": "forward", "to": to}
	case "markRead", "star":
		normalized := map[string]any{"type": kind}
		if flag, present := action["value"]; present {
			normalized["value"] = flag
		}
		return normalized
	}
	return map[string]any{"type": TextValue{}.Of(kind)}
}
