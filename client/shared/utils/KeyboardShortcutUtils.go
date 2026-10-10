import (
	"cmp"
	"maps"
	"regexp"
	"slices"
	"strings"
)

// KeyboardShortcutUtils is the settings side of shared/scripts/keyboard.js:
// it normalizes portable key specifications, validates a set of overrides
// against the command catalog and writes bindings for people to read. The
// key listener itself stays in the page.
type KeyboardShortcutUtils struct{}

// NormalizeBinding answers the canonical form of a key specification, "off"
// for a disabled command, or nothing when the specification is not valid.
func (utils KeyboardShortcutUtils) NormalizeBinding(value string) string {
	source := utils.collapse(value)
	chords := strings.Split(source, utils.space())
	normalized := slices.DeleteFunc([]string{utils.chord(chords, 0), utils.chord(chords, 1)}, func(chord string) bool { return chord == "" })
	return cmp.Or(utils.disabledSource(source), utils.accepted(source, normalized, len(chords)))
}

// Validate checks overrides against the catalog: every command must exist,
// every binding must parse, and no two commands may share a key in a context.
func (utils KeyboardShortcutUtils) Validate(catalog []KeymapAction, overrides map[string]string, apple bool) ShortcutValidation {
	normalized := utils.normalized(overrides)
	bindings := utils.bindings(catalog, normalized)
	problem := cmp.Or(utils.problem(catalog, overrides), utils.conflict(catalog, bindings, apple))
	return ShortcutValidation{Valid: problem == "", Problem: problem, Overrides: utils.changed(catalog, normalized)}
}

// Display writes a binding as the platform shows keys, such as "Ctrl+Enter"
// or "G then I".
func (utils KeyboardShortcutUtils) display(binding string, apple bool) string {
	names := utils.names(apple)
	text := regexp.MustCompile(`[^+ ]+`).ReplaceAllStringFunc(binding, func(part string) string { return cmp.Or(names[part], utils.upper(part)) })
	joined := strings.ReplaceAll(strings.ReplaceAll(text, utils.plus(), FlowUtils{}.Text(apple, "", utils.plus())), utils.space(), " then ")
	return cmp.Or(utils.disabled(binding), joined)
}

// Groups arranges the catalog for the editor, in catalog order.
func (utils KeyboardShortcutUtils) Groups(catalog []KeymapAction, draft map[string]string, apple bool) []ShortcutGroup {
	return utils.grouped(utils.labels(catalog), utils.rows(catalog, draft, apple))
}

// Disabled is the binding that turns a command off.
func (KeyboardShortcutUtils) Disabled() string { return "off" }

func (KeyboardShortcutUtils) space() string { return " " }

func (KeyboardShortcutUtils) sentence(text string) string { return text + "." }

func (KeyboardShortcutUtils) plus() string { return "+" }

func (KeyboardShortcutUtils) longestBinding() int { return 64 }

func (KeyboardShortcutUtils) longestSequence() int { return 2 }

func (utils KeyboardShortcutUtils) collapse(value string) string {
	text := regexp.MustCompile(`\s*\+\s*`).ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), utils.plus())
	return regexp.MustCompile(`\s+`).ReplaceAllString(text, utils.space())
}

func (utils KeyboardShortcutUtils) chord(chords []string, index int) string {
	return utils.canonical(strings.Join(chords[min(index, len(chords)):min(index+1, len(chords))], ""))
}

func (utils KeyboardShortcutUtils) canonical(chord string) string {
	aliases := utils.aliases()
	parts := strings.Split(strings.NewReplacer(aliases...).Replace(chord), utils.plus())
	modifiers := slices.DeleteFunc(slices.Clone(parts), func(part string) bool { return !utils.modifiers()[part] })
	key := strings.Join(slices.DeleteFunc(slices.Clone(parts), func(part string) bool { return utils.modifiers()[part] }), utils.plus())
	slices.Sort(modifiers)
	distinct := len(slices.Compact(slices.Clone(modifiers))) == len(modifiers)
	if chord != "" && !slices.Contains(parts, "") && distinct && (utils.namedKeys()[key] || regexp.MustCompile(`^[a-z0-9?!*#/,._-]$`).MatchString(key)) {
		return strings.Join(append(modifiers, key), utils.plus())
	}
	return ""
}

func (utils KeyboardShortcutUtils) disabledSource(source string) string {
	if source == utils.Disabled() {
		return source
	}
	return ""
}

func (utils KeyboardShortcutUtils) accepted(source string, normalized []string, chords int) string {
	if source != "" && len(source) <= utils.longestBinding() && chords <= utils.longestSequence() && len(normalized) == chords {
		return strings.Join(normalized, utils.space())
	}
	return ""
}

// aliases are replacement pairs from the spellings people write to the
// canonical modifier and key names.
func (utils KeyboardShortcutUtils) aliases() []string {
	return []string{"control", utils.control(), "option", utils.alternate(), "command", utils.meta(), "cmd", utils.meta(), "escape", utils.escape()}
}

func (KeyboardShortcutUtils) control() string { return "ctrl" }

func (KeyboardShortcutUtils) alternate() string { return "alt" }

func (KeyboardShortcutUtils) meta() string { return "meta" }

func (KeyboardShortcutUtils) escape() string { return "esc" }

func (KeyboardShortcutUtils) portable() string { return "mod" }

func (KeyboardShortcutUtils) shift() string { return "shift" }

func (utils KeyboardShortcutUtils) modifiers() map[string]bool {
	return map[string]bool{utils.control(): true, utils.alternate(): true, utils.meta(): true, utils.portable(): true, utils.shift(): true}
}

func (utils KeyboardShortcutUtils) namedKeys() map[string]bool {
	return map[string]bool{"backspace": true, "delete": true, "down": true, "enter": true, utils.escape(): true, "left": true, "right": true, "space": true, "tab": true, "up": true}
}

// normalized reads each submitted override; an empty field disables the command.
func (utils KeyboardShortcutUtils) normalized(overrides map[string]string) map[string]string {
	normalized := map[string]string{}
	for id, value := range overrides {
		normalized[id] = utils.NormalizeBinding(cmp.Or(value, utils.Disabled()))
	}
	return normalized
}

func (KeyboardShortcutUtils) bindings(catalog []KeymapAction, normalized map[string]string) map[string]string {
	bindings := map[string]string{}
	for _, action := range catalog {
		bindings[action.ID] = cmp.Or(normalized[action.ID], action.Binding)
	}
	return bindings
}

// changed keeps only the overrides that differ from each command's default.
func (KeyboardShortcutUtils) changed(catalog []KeymapAction, normalized map[string]string) map[string]string {
	defaults := map[string]string{}
	for _, action := range catalog {
		defaults[action.ID] = action.Binding
	}
	changed := maps.Clone(normalized)
	maps.DeleteFunc(changed, func(id string, binding string) bool { return binding == defaults[id] })
	return changed
}

// problem names the first unknown command or unreadable binding, in catalog order.
func (utils KeyboardShortcutUtils) problem(catalog []KeymapAction, overrides map[string]string) string {
	labels, order := utils.positions(catalog)
	ids := slices.SortedFunc(maps.Keys(overrides), func(left string, right string) int { return cmp.Compare(order[left], order[right]) })
	problems := []string{}
	for _, id := range ids {
		problems = append(problems, cmp.Or(
			FlowUtils{}.Text(labels[id] == "", "Unknown keyboard command: "+id, ""),
			FlowUtils{}.Text(utils.NormalizeBinding(cmp.Or(overrides[id], utils.Disabled())) == "", utils.sentence("Invalid shortcut for "+labels[id]), ""),
		))
	}
	return cmp.Or(problems...)
}

func (KeyboardShortcutUtils) positions(catalog []KeymapAction) (map[string]string, map[string]int) {
	labels := map[string]string{}
	order := map[string]int{}
	for index, action := range catalog {
		labels[action.ID] = action.Label
		order[action.ID] = index + 1
	}
	return labels, order
}

// conflict names the first two commands that share a binding in one context.
func (utils KeyboardShortcutUtils) conflict(catalog []KeymapAction, bindings map[string]string, apple bool) string {
	contexts := map[string]int{"global": 63, "mail": 14, "reader": 12, "reader-find": 8, "reader-no-find": 4, "mailbox": 2, "composer": 16, "calendar": 32}
	seen := map[string]int{}
	first := map[string]string{}
	conflict := ""
	for _, action := range catalog {
		binding := bindings[action.ID]
		mask := contexts[action.Scope] * map[bool]int{true: 0, false: 1}[binding == utils.Disabled()]
		message := utils.sentence(first[binding] + " and " + action.Label + " both use " + utils.display(binding, apple))
		conflict = cmp.Or(conflict, FlowUtils{}.Text(seen[binding]&mask != 0, message, ""))
		first[binding] = cmp.Or(first[binding], action.Label)
		seen[binding] |= mask
	}
	return conflict
}

func (utils KeyboardShortcutUtils) names(apple bool) map[string]string {
	control, command := "Ctrl", "⌘"
	names := map[string]string{utils.control(): control, "up": "↑", "down": "↓", "left": "←", "right": "→", utils.escape(): "Esc", "enter": "Enter"}
	maps.Copy(names, map[bool]map[string]string{
		true:  {utils.portable(): command, utils.meta(): command, utils.alternate(): "⌥", utils.shift(): "⇧"},
		false: {utils.portable(): control, utils.meta(): "Meta", utils.alternate(): "Alt", utils.shift(): "Shift"},
	}[apple])
	return names
}

func (KeyboardShortcutUtils) upper(part string) string {
	return FlowUtils{}.Text(len(part) == 1, strings.ToUpper(part), part)
}

func (utils KeyboardShortcutUtils) disabled(binding string) string {
	if binding == "" || binding == utils.Disabled() {
		return "Disabled"
	}
	return ""
}

func (KeyboardShortcutUtils) labels(catalog []KeymapAction) []string {
	labels := []string{}
	for _, action := range catalog {
		labels = append(labels, action.Group)
	}
	seen := map[string]bool{}
	return slices.DeleteFunc(labels, func(label string) bool {
		repeated := seen[label]
		seen[label] = true
		return repeated
	})
}

func (utils KeyboardShortcutUtils) rows(catalog []KeymapAction, draft map[string]string, apple bool) map[string][]ShortcutRow {
	rows := map[string][]ShortcutRow{}
	for _, action := range catalog {
		binding := cmp.Or(draft[action.ID], action.Binding)
		rows[action.Group] = append(rows[action.Group], ShortcutRow{
			ID: action.ID, Label: action.Label, Binding: binding,
			InputBinding: FlowUtils{}.Text(binding == utils.Disabled(), "", binding),
			DefaultLabel: utils.display(action.Binding, apple),
		})
	}
	return rows
}

func (KeyboardShortcutUtils) grouped(labels []string, rows map[string][]ShortcutRow) []ShortcutGroup {
	groups := []ShortcutGroup{}
	for _, label := range labels {
		groups = append(groups, ShortcutGroup{Label: label, Rows: rows[label]})
	}
	return groups
}
