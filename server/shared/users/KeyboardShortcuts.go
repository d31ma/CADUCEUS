import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// KeyboardShortcuts keeps a user's keyboard shortcut overrides. Stored rows
// are read forgivingly, but an invalid or conflicting override is never kept:
// each one is admitted only while the whole keymap stays unambiguous.
type KeyboardShortcuts struct{}

// Overrides answers the valid overrides of value, keyed by command id.
func (shortcuts KeyboardShortcuts) Overrides(value any) map[string]any {
	input, isObject := RecordValues{}.Object(value)
	accepted := map[string]string{}
	if !isObject {
		return map[string]any{}
	}
	actions := shortcuts.actions()
	for _, action := range actions {
		raw, present := input[action.Id]
		if !present {
			continue
		}
		if raw == "" {
			raw = "off"
		}
		binding := shortcuts.Binding(raw)
		if binding == "" || binding == action.Binding {
			continue
		}
		candidate := map[string]string{action.Id: binding}
		for id, other := range accepted {
			candidate[id] = other
		}
		if !shortcuts.conflicting(actions, shortcuts.resolved(actions, candidate)) {
			accepted = candidate
		}
	}
	overrides := map[string]any{}
	for id, binding := range accepted {
		overrides[id] = binding
	}
	return overrides
}

// Binding answers one portable shortcut in its normal form, "off", or ""
// when its grammar is unsafe.
func (shortcuts KeyboardShortcuts) Binding(value any) string {
	text, isText := value.(string)
	if !isText {
		return ""
	}
	source := regexp.MustCompile(`\s*\+\s*`).ReplaceAllString(strings.ToLower(Security{}.Trim(text)), "+")
	source = regexp.MustCompile(`\s+`).ReplaceAllString(source, " ")
	if source == "" || len([]rune(source)) > shortcuts.maximumBindingLength() {
		return ""
	}
	if source == "off" {
		return source
	}
	sequence := strings.Split(source, " ")
	if len(sequence) > shortcuts.maximumSequenceLength() {
		return ""
	}
	normalized := []string{}
	for _, chord := range sequence {
		formed := shortcuts.chord(chord)
		if formed == "" {
			return ""
		}
		normalized = append(normalized, formed)
	}
	return strings.Join(normalized, " ")
}

func (shortcuts KeyboardShortcuts) chord(chord string) string {
	modifiers := []string{}
	key := ""
	for _, raw := range strings.Split(chord, "+") {
		piece := strings.TrimSpace(raw)
		if piece == "" {
			return ""
		}
		if alias, aliased := shortcuts.modifierAliases()[piece]; aliased {
			piece = alias
		}
		if shortcuts.isModifier(piece) {
			if (PreferenceChoices{}).Includes(modifiers, piece) {
				return ""
			}
			modifiers = append(modifiers, piece)
			continue
		}
		if key != "" {
			return ""
		}
		key = piece
		if piece == "escape" {
			key = "esc"
		}
	}
	if key == "" || !shortcuts.isKey(key) {
		return ""
	}
	sort.Strings(modifiers)
	return strings.Join(append(modifiers, key), "+")
}

func (KeyboardShortcuts) isModifier(piece string) bool {
	return PreferenceChoices{}.Includes([]string{"alt", "ctrl", "meta", "mod", "shift"}, piece)
}

func (KeyboardShortcuts) isKey(key string) bool {
	named := []string{"backspace", "delete", "down", "enter", "esc", "left", "right", "space", "tab", "up"}
	return PreferenceChoices{}.Includes(named, key) || regexp.MustCompile(`^[a-z0-9?!*#/,._-]$`).MatchString(key)
}

func (KeyboardShortcuts) modifierAliases() map[string]string {
	return map[string]string{"control": "ctrl", "option": "alt", "cmd": "meta", "command": "meta"}
}

func (KeyboardShortcuts) maximumBindingLength() int { return 64 }

func (KeyboardShortcuts) maximumSequenceLength() int { return 2 }

// actions answers the catalog with its bindings in normal form.
func (shortcuts KeyboardShortcuts) actions() []KeyboardAction {
	var actions []KeyboardAction
	_ = json.Unmarshal([]byte(KeyboardCatalog{}.Text()), &actions)
	for index := range actions {
		actions[index].Binding = shortcuts.Binding(actions[index].Binding)
	}
	return actions
}

func (KeyboardShortcuts) resolved(actions []KeyboardAction, overrides map[string]string) map[string]string {
	bindings := map[string]string{}
	for _, action := range actions {
		bindings[action.Id] = action.Binding
		if override := overrides[action.Id]; override != "" {
			bindings[action.Id] = override
		}
	}
	return bindings
}

// conflicting reports whether two commands that can be active together share
// one binding.
func (shortcuts KeyboardShortcuts) conflicting(actions []KeyboardAction, bindings map[string]string) bool {
	return len(shortcuts.conflict(actions, bindings)) > 0
}

// conflict answers the first two commands that can be active together and
// share one binding, or none.
func (shortcuts KeyboardShortcuts) conflict(actions []KeyboardAction, bindings map[string]string) []KeyboardAction {
	for leftIndex, left := range actions {
		binding := bindings[left.Id]
		if binding == "" || binding == "off" {
			continue
		}
		for _, right := range actions[leftIndex+1:] {
			if bindings[right.Id] == binding && shortcuts.sharesContext(left.Scope, right.Scope) {
				return []KeyboardAction{left, right}
			}
		}
	}
	return nil
}

// Checked answers the overrides a caller sent, or why they are refused. An
// empty binding disables the command; a default binding is not stored.
func (shortcuts KeyboardShortcuts) Checked(value any) (map[string]any, *PreferenceProblem) {
	input, isObject := RecordValues{}.Object(value)
	if !isObject {
		return nil, &PreferenceProblem{Message: "keyboardShortcutOverrides must be an object"}
	}
	actions := shortcuts.actions()
	if len(input) > len(actions) {
		return nil, &PreferenceProblem{Message: fmt.Sprintf("keyboardShortcutOverrides may contain at most %d commands", len(actions))}
	}
	byId := map[string]KeyboardAction{}
	for _, action := range actions {
		byId[action.Id] = action
	}
	accepted := map[string]string{}
	for _, id := range (PreferenceChoices{}).Keys(input) {
		action, known := byId[id]
		if !known {
			return nil, &PreferenceProblem{Message: "Unknown keyboard command: " + id}
		}
		raw := input[id]
		if raw == "" {
			raw = "off"
		}
		binding := shortcuts.Binding(raw)
		if binding == "" {
			return nil, &PreferenceProblem{Message: "Invalid shortcut for " + action.Label}
		}
		if binding != action.Binding {
			accepted[id] = binding
		}
	}
	bindings := shortcuts.resolved(actions, accepted)
	if pair := shortcuts.conflict(actions, bindings); len(pair) > 0 {
		return nil, &PreferenceProblem{Message: pair[0].Label + " and " + pair[1].Label + " both use " + bindings[pair[0].Id]}
	}
	overrides := map[string]any{}
	for id, binding := range accepted {
		overrides[id] = binding
	}
	return overrides, nil
}

func (shortcuts KeyboardShortcuts) sharesContext(left string, right string) bool {
	rightContexts := shortcuts.contexts(right)
	for _, context := range shortcuts.contexts(left) {
		if (PreferenceChoices{}).Includes(rightContexts, context) {
			return true
		}
	}
	return false
}

func (KeyboardShortcuts) contexts(scope string) []string {
	switch scope {
	case "global":
		return []string{"other", "mailbox", "reader", "reader-find", "composer", "calendar"}
	case "mail":
		return []string{"mailbox", "reader", "reader-find"}
	case "reader":
		return []string{"reader", "reader-find"}
	case "reader-no-find":
		return []string{"reader"}
	}
	return []string{scope}
}
