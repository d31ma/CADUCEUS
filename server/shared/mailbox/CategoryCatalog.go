import (
	"sort"
	"strings"
)

// CategoryCatalog keeps the folders and labels a user created, per assigned
// domain: at most 200 safe paths each, each one a folder or a label.
type CategoryCatalog struct{}

// Normalized answers the catalog of every assigned domain.
func (catalog CategoryCatalog) Normalized(value any, assignedDomains []string) map[string]any {
	source, _ := RecordValues{}.Object(value)
	normalized := map[string]any{}
	for _, rawDomain := range assignedDomains {
		domain := Security{}.NormalizeDomain(rawDomain)
		if domain == "" {
			continue
		}
		entries, _ := RecordValues{}.Object(source[domain])
		normalized[domain] = catalog.entries(entries)
	}
	return normalized
}

func (catalog CategoryCatalog) entries(entries map[string]any) map[string]any {
	type entry struct{ path, kind string }
	valid := []entry{}
	for rawPath, rawKind := range entries {
		kindText, _ := rawKind.(string)
		kind := strings.ToLower(kindText)
		path := CategoryPath{}.Normalized(rawPath)
		if path != "" && (kind == "folder" || kind == "label") {
			valid = append(valid, entry{path: path, kind: kind})
		}
	}
	sort.SliceStable(valid, func(left, right int) bool {
		return TextOrder{}.Compare(valid[left].path, valid[right].path) < 0
	})
	if len(valid) > catalog.maximumEntries() {
		valid = valid[:catalog.maximumEntries()]
	}
	kept := map[string]any{}
	for _, item := range valid {
		kept[item.path] = item.kind
	}
	return kept
}

func (CategoryCatalog) maximumEntries() int { return 200 }
