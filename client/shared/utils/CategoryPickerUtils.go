import (
	"regexp"
	"slices"
	"strings"
)

// CategoryPickerUtils builds the rows of the move-to-folder and apply-labels
// picker for one mailbox account, the way Mailspring does.
type CategoryPickerUtils struct{}

// NormalizePath matches the server's bounded category-path grammar, or gives
// "" for a path the server would refuse.
func (CategoryPickerUtils) NormalizePath(value string) string {
	path := strings.ToLower(strings.TrimSpace(value))
	return CategoryPickerUtils{}.pathUnlessRefused(path, slices.ContainsFunc(strings.Split(path, "/"), CategoryPickerUtils{}.isDotPart))
}

// Items lists the account's categories of one kind that match the search,
// with how many selected messages carry each label.
func (CategoryPickerUtils) Items(account CategoryAccount, kind string, search string, messages []PickerMessage) []CategoryPickerItem {
	query := strings.ToLower(strings.TrimSpace(search))
	return CategoryPickerUtils{}.itemsWhenCapable(account, kind, CategoryPickerUtils{}.matching(account.Categories, kind, query), messages)
}

// LabelPatch is the mail action for a label click: a label every selected
// message has is removed, any other is added to all of them.
func (CategoryPickerUtils) LabelPatch(messages []PickerMessage, path string) BehaviorValue {
	remove := CategoryPickerUtils{}.state(messages, path) == CategoryPickerUtils{}.every()
	both := [][]string{{}, {path}}
	return map[string]any{"labels": map[string]any{"add": both[CategoryPickerUtils{}.flag(!remove)], "remove": both[CategoryPickerUtils{}.flag(remove)]}}
}

// SingleDomain is the one mailbox domain all messages belong to, or "".
func (CategoryPickerUtils) SingleDomain(messages []PickerMessage) string {
	return CategoryPickerUtils{}.onlyOne(CategoryPickerUtils{}.domains(messages))
}

func (CategoryPickerUtils) domains(messages []PickerMessage) []string {
	domains := []string{}
	for _, message := range messages {
		domains = append(domains, strings.ToLower(strings.TrimSpace(message.Domain)))
	}
	return slices.Compact(slices.Sorted(slices.Values(slices.DeleteFunc(domains, func(domain string) bool { return domain == "" }))))
}

func (CategoryPickerUtils) onlyOne(domains []string) string {
	if len(domains) != 1 {
		return ""
	}
	return domains[0]
}

func (CategoryPickerUtils) isDotPart(part string) bool {
	if part == "." || part == ".." {
		return true
	}
	return false
}

func (CategoryPickerUtils) pathUnlessRefused(path string, dotted bool) string {
	if !regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._/-]{0,58}[a-z0-9])?$`).MatchString(path) || strings.Contains(path, "//") || dotted {
		return ""
	}
	return path
}

func (utils CategoryPickerUtils) matching(categories []Category, kind string, query string) []Category {
	return slices.DeleteFunc(slices.Clone(categories), utils.excluded(kind, query))
}

func (CategoryPickerUtils) excluded(kind string, query string) func(Category) bool {
	return func(category Category) bool {
		if category.Kind != kind || (query != "" && !strings.Contains(strings.ToLower(category.Path), query) && !strings.Contains(strings.ToLower(category.DisplayName), query)) {
			return true
		}
		return false
	}
}

func (utils CategoryPickerUtils) itemsWhenCapable(account CategoryAccount, kind string, categories []Category, messages []PickerMessage) []CategoryPickerItem {
	if !map[string]bool{utils.labelKind(): account.Capabilities.Labels, "folder": account.Capabilities.Folders}[kind] {
		return []CategoryPickerItem{}
	}
	return utils.items(kind, categories, messages)
}

func (utils CategoryPickerUtils) items(kind string, categories []Category, messages []PickerMessage) []CategoryPickerItem {
	items := []CategoryPickerItem{}
	for _, category := range categories {
		items = append(items, CategoryPickerItem{
			Path: category.Path, Kind: kind, DisplayName: utils.displayName(category), Depth: int(max(0, category.Depth)),
			ParentPath: category.ParentPath, Role: category.Role,
			SelectedState: map[bool]string{true: utils.state(messages, category.Path), false: utils.unlabelled()}[kind == utils.labelKind()],
		})
	}
	return items
}

func (CategoryPickerUtils) displayName(category Category) string {
	if category.DisplayName == "" {
		return category.Path
	}
	return category.DisplayName
}

func (utils CategoryPickerUtils) state(messages []PickerMessage, path string) string {
	usage := len(slices.DeleteFunc(slices.Clone(messages), func(message PickerMessage) bool { return !slices.Contains(message.Labels, path) }))
	every := map[bool]int{true: 1, false: 0}[usage == len(messages)]
	return []string{utils.unlabelled(), "mixed", utils.every()}[min(usage, 1)*(1+every)]
}

func (CategoryPickerUtils) flag(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (CategoryPickerUtils) every() string { return "all" }

func (CategoryPickerUtils) unlabelled() string { return "none" }

func (CategoryPickerUtils) labelKind() string { return "label" }
