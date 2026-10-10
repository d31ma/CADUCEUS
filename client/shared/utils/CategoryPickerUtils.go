import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// CategoryPickerUtils builds the folder and label picker rows for one
// message, as category-picker.js did.
type CategoryPickerUtils struct{}

func (CategoryPickerUtils) NormalizePickerPath(value string) string {
	path := strings.ToLower(strings.TrimSpace(value))
	return CategoryPickerUtils{}.validPath(path)
}

func (CategoryPickerUtils) BuildItems(account CategoryAccount, kind string, search string, labels []string) []CategoryPickerItem {
	query := strings.ToLower(strings.TrimSpace(search))
	return CategoryPickerUtils{}.items(account, kind, query, labels)
}

func (CategoryPickerUtils) allSelected() string { return "all" }

func (CategoryPickerUtils) LabelPatch(labels []string, path string) map[string]any {
	remove := CategoryPickerUtils{}.selectionState(labels, path) == CategoryPickerUtils{}.allSelected()
	return map[string]any{"labels": CategoryPickerUtils{}.labelChange(remove, path)}
}

func (CategoryPickerUtils) categoryPath() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._/-]{0,58}[a-z0-9])?$`)
}

func (CategoryPickerUtils) validPath(path string) string {
	parts := strings.Split(path, "/")
	valid := CategoryPickerUtils{}.categoryPath().MatchString(path)
	if !valid || strings.Contains(path, "//") || slices.Contains(parts, ".") || slices.Contains(parts, "..") {
		return ""
	}
	return path
}

func (CategoryPickerUtils) items(account CategoryAccount, kind string, query string, labels []string) []CategoryPickerItem {
	if !CategoryPickerUtils{}.supports(account.Capabilities, kind) {
		return []CategoryPickerItem{}
	}
	return CategoryPickerUtils{}.matchingItems(account.Categories, kind, query, labels)
}

func (CategoryPickerUtils) labelKind() string { return "label" }

func (CategoryPickerUtils) supports(capabilities CategoryCapabilities, kind string) bool {
	if kind == CategoryPickerUtils{}.labelKind() {
		return capabilities.Labels
	}
	return capabilities.Folders
}

func (CategoryPickerUtils) matchingItems(categories []Category, kind string, query string, labels []string) []CategoryPickerItem {
	items := []CategoryPickerItem{}
	for _, category := range categories {
		items = append(items, CategoryPickerUtils{}.matchingItem(category, kind, query, labels)...)
	}
	return items
}

func (CategoryPickerUtils) matchingItem(category Category, kind string, query string, labels []string) []CategoryPickerItem {
	if category.Kind != kind || (query != "" && !strings.Contains(strings.ToLower(category.Path), query) && !strings.Contains(strings.ToLower(category.DisplayName), query)) {
		return []CategoryPickerItem{}
	}
	return []CategoryPickerItem{CategoryPickerUtils{}.item(category, kind, labels)}
}

func (CategoryPickerUtils) maximumDepth() int { return 6 }

func (CategoryPickerUtils) item(category Category, kind string, labels []string) CategoryPickerItem {
	state := CategoryPickerUtils{}.itemState(kind, labels, category.Path)
	depth := max(0, category.Depth)
	return CategoryPickerItem{
		Path:          category.Path,
		Kind:          kind,
		DisplayName:   CategoryPickerUtils{}.displayName(category),
		Depth:         depth,
		ParentPath:    category.ParentPath,
		Role:          category.Role,
		SelectedState: state,
		Indent:        "padding-inline-start:" + strconv.Itoa(12+min(CategoryPickerUtils{}.maximumDepth(), depth)*18) + "px",
		Check:         CategoryPickerUtils{}.check(state),
		AriaSelected:  strconv.FormatBool(state == CategoryPickerUtils{}.allSelected()),
	}
}

func (CategoryPickerUtils) displayName(category Category) string {
	if category.DisplayName == "" {
		return category.Path
	}
	return category.DisplayName
}

func (CategoryPickerUtils) noneSelected() string { return "none" }

func (CategoryPickerUtils) itemState(kind string, labels []string, path string) string {
	if kind != CategoryPickerUtils{}.labelKind() {
		return CategoryPickerUtils{}.noneSelected()
	}
	return CategoryPickerUtils{}.selectionState(labels, path)
}

func (CategoryPickerUtils) selectionState(labels []string, path string) string {
	if slices.Contains(labels, path) {
		return CategoryPickerUtils{}.allSelected()
	}
	return CategoryPickerUtils{}.noneSelected()
}

func (CategoryPickerUtils) check(state string) string {
	switch state {
	case CategoryPickerUtils{}.allSelected():
		return "✓"
	case "mixed":
		return "−"
	}
	return ""
}

func (CategoryPickerUtils) labelChange(remove bool, path string) map[string]any {
	changes := map[bool][2][]string{true: {{}, {path}}, false: {{path}, {}}}
	change := changes[remove]
	return map[string]any{"add": change[0], "remove": change[1]}
}
