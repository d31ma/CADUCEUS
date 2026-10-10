import (
	"encoding/json"
	"sort"
	"strings"
)

// RulesRecords converts inbox rule documents. FYLO stores a rule's
// conditions and actions as JSON text; routes see them as lists.
type RulesRecords struct{}

// Deserialize answers one stored rule as routes see it, with its owner and
// mailbox lower-cased, a numeric order and decoded conditions and actions.
func (records RulesRecords) Deserialize(raw map[string]any) (map[string]any, error) {
	text := TextValue{}
	rule := text.Copy(raw)
	for _, field := range []string{"ownerEmail", "mailbox"} {
		if text.Truthy(raw[field]) {
			rule[field] = strings.ToLower(text.Of(raw[field]))
		}
	}
	rule["order"] = text.Number(raw["order"])
	rule["stopProcessing"] = text.Truthy(raw["stopProcessing"])
	for _, field := range []string{"conditions", "actions"} {
		decoded, err := records.decoded(raw[field])
		if err != nil {
			return nil, err
		}
		rule[field] = decoded
	}
	return rule, nil
}

func (RulesRecords) decoded(value any) (any, error) {
	if encoded, isText := value.(string); isText {
		var decoded any
		err := json.Unmarshal([]byte(encoded), &decoded)
		return decoded, err
	}
	if value == nil {
		return []any{}, nil
	}
	return value, nil
}

// CompareRules orders rules by order, then name, then id.
func (RulesRecords) CompareRules(left map[string]any, right map[string]any) int {
	text := TextValue{}
	if difference := text.Number(left["order"]) - text.Number(right["order"]); difference != 0 {
		return map[bool]int{true: -1, false: 1}[difference < 0]
	}
	order := LocaleOrder{}
	if byName := order.Compare(text.Of(left["name"]), text.Of(right["name"])); byName != 0 {
		return byName
	}
	return order.Compare(text.Of(left["id"]), text.Of(right["id"]))
}

// Serialize answers a rule document with its conditions and actions as
// JSON text.
func (RulesRecords) Serialize(rule map[string]any) map[string]any {
	text := TextValue{}
	stored := text.Copy(rule)
	for _, field := range []string{"conditions", "actions"} {
		value := rule[field]
		if value == nil {
			value = []any{}
		}
		stored[field] = text.Json(value)
	}
	return stored
}

// SerializePatch answers the stored fields of a rule patch: only the fields
// a rule may change, with conditions and actions as JSON text.
func (RulesRecords) SerializePatch(patch map[string]any) map[string]any {
	text := TextValue{}
	serialized := map[string]any{}
	for _, field := range []string{"name", "enabled", "ownerEmail", "mailbox", "order", "stopProcessing", "conditionMatch"} {
		if value, present := patch[field]; present {
			serialized[field] = value
		}
	}
	for _, field := range []string{"conditions", "actions"} {
		if value, present := patch[field]; present {
			serialized[field] = text.Json(value)
		}
	}
	return serialized
}

// Sorted answers the deserialized rules of the stored records that keep
// passes, in rule order.
func (records RulesRecords) Sorted(stored []FyloRecord, keep func(raw map[string]any) bool) ([]map[string]any, error) {
	rules := []map[string]any{}
	for _, record := range stored {
		if !keep(record.Data) {
			continue
		}
		rule, err := records.Deserialize(record.Data)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	records.Sort(rules)
	return rules, nil
}

// Sort orders rules in place, keeping equal rules in their order.
func (records RulesRecords) Sort(rules []map[string]any) {
	sort.SliceStable(rules, func(left, right int) bool { return records.CompareRules(rules[left], rules[right]) < 0 })
}

// ForMailbox answers the rules that apply to one mailbox: those of its
// exact owner (for any mailbox or this one), and historical rules without
// an owner, which stay domain-wide.
func (records RulesRecords) ForMailbox(rules []map[string]any, ownerEmail string, mailbox string) []map[string]any {
	text := TextValue{}
	owner := strings.ToLower(text.Trim(ownerEmail))
	address := strings.ToLower(text.Trim(mailbox))
	kept := []map[string]any{}
	for _, rule := range rules {
		if !text.Truthy(rule["ownerEmail"]) || (rule["ownerEmail"] == owner && (!text.Truthy(rule["mailbox"]) || rule["mailbox"] == address)) {
			kept = append(kept, rule)
		}
	}
	records.Sort(kept)
	return kept
}

// NextOrder answers the order after the highest order of rules.
func (RulesRecords) NextOrder(rules []map[string]any) float64 {
	highest := -1.0
	for _, rule := range rules {
		highest = max(highest, TextValue{}.Number(rule["order"]))
	}
	return highest + 1
}

// SameRules reports whether ids name each of an owner's rules exactly once.
func (RulesRecords) SameRules(rules []map[string]any, ids []string) bool {
	known := map[string]bool{}
	for _, rule := range rules {
		known[TextValue{}.Of(rule["id"])] = true
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] || !known[id] {
			return false
		}
		seen[id] = true
	}
	return len(ids) == len(rules)
}
