import (
	"sort"
	"strings"
)

// CategoryAccounts presents the physical folders and labels of each exact
// mailbox an owner holds, with the system role each folder carries, and adds
// the folders and labels an owner creates.
type CategoryAccounts struct{}

// Listed answers one account per allowed domain, sorted by domain, from the
// owner's user record and the owner's own emails.
func (accounts CategoryAccounts) Listed(user map[string]any, allowedDomains []string, emails []map[string]any) []map[string]any {
	mappings := CategoryMappings{}.Of(user)
	domains, _ := user["domains"].([]string)
	catalog := CategoryCatalog{}.Normalized(user["categoryCatalog"], domains)
	sorted := append([]string{}, allowedDomains...)
	sort.Strings(sorted)
	listed := []map[string]any{}
	for _, domain := range sorted {
		listed = append(listed, accounts.account(mappings, domain, catalog, emails))
	}
	return listed
}

func (accounts CategoryAccounts) account(mappings CategoryMappings, domain string, catalog map[string]any, emails []map[string]any) map[string]any {
	text := TextValue{}
	assigned := mappings.ForDomain(domain)
	folders := []string{}
	labels := []string{}
	for _, role := range mappings.Roles() {
		folders = append(folders, assigned[role])
	}
	for _, email := range emails {
		if text.Of(email["domain"]) != domain {
			continue
		}
		if path := (CategoryPath{}).Normalized(text.Or(email["folder"], "inbox")); path != "" {
			folders = append(folders, path)
		}
		for _, label := range text.List(email["labels"]) {
			labels = append(labels, text.Of(label))
		}
	}
	entries, _ := catalog[domain].(map[string]any)
	for path, kind := range entries {
		if kind == "label" {
			labels = append(labels, path)
		} else {
			folders = append(folders, path)
		}
	}
	roleByPath := mappings.RoleByPath(assigned)
	categories := []map[string]any{}
	for _, path := range (MailAddresses{}).Unique(folders) {
		categories = append(categories, accounts.Present(path, "folder", roleByPath[path]))
	}
	for _, path := range (MailAddresses{}).Unique(labels) {
		categories = append(categories, accounts.Present(path, "label", ""))
	}
	order := TextOrder{}
	sort.SliceStable(categories, func(left, right int) bool {
		if kind := order.Compare(text.Of(categories[left]["kind"]), text.Of(categories[right]["kind"])); kind != 0 {
			return kind < 0
		}
		return order.Compare(text.Of(categories[left]["path"]), text.Of(categories[right]["path"])) < 0
	})
	return map[string]any{
		"domain":       domain,
		"mappings":     assigned,
		"capabilities": map[string]any{"folders": true, "labels": true, "create": true},
		"categories":   categories,
	}
}

// Present answers one category with its hierarchy: display name, parent
// path, depth and, for a mapped folder, its role.
func (CategoryAccounts) Present(path string, kind string, role string) map[string]any {
	parts := strings.Split(path, "/")
	leaf := parts[len(parts)-1]
	if leaf == "" {
		leaf = path
	}
	category := map[string]any{"path": path, "kind": kind, "displayName": leaf, "depth": len(parts) - 1}
	if leaf != "" {
		first := []rune(leaf)
		category["displayName"] = strings.ToUpper(string(first[0])) + string(first[1:])
	}
	if len(parts) > 1 {
		category["parentPath"] = strings.Join(parts[:len(parts)-1], "/")
	}
	if role != "" {
		category["role"] = role
	}
	return category
}

// Added validates one user-created folder or label and answers the owner's
// new catalog and the category, or the refusal: forbidden for a domain the
// owner is not assigned, otherwise a bad request.
func (accounts CategoryAccounts) Added(user map[string]any, input any) (map[string]any, map[string]any, *CategoryRefusal) {
	body, isObject := RecordValues{}.Object(input)
	if !isObject {
		return nil, nil, &CategoryRefusal{Message: "Category payload required"}
	}
	assigned, _ := user["domains"].([]string)
	domain := Security{}.NormalizeDomain(RecordValues{}.Text(body["domain"]))
	allowed := false
	for _, candidate := range assigned {
		allowed = allowed || (domain != "" && Security{}.NormalizeDomain(candidate) == domain)
	}
	if !allowed {
		return nil, nil, &CategoryRefusal{Forbidden: true, Message: "Category domain is not assigned to this user"}
	}
	kind := strings.ToLower(RecordValues{}.Text(body["kind"]))
	path := CategoryPath{}.Normalized(body["path"])
	if (kind != "folder" && kind != "label") || path == "" {
		return nil, nil, &CategoryRefusal{Message: "kind and a valid category path are required"}
	}
	catalog := CategoryCatalog{}.Normalized(user["categoryCatalog"], assigned)
	entries, _ := catalog[domain].(map[string]any)
	if entries == nil {
		entries = map[string]any{}
	}
	if existing, found := entries[path]; found {
		if existing != kind {
			return nil, nil, &CategoryRefusal{Message: "A category path cannot be both a folder and a label"}
		}
		return catalog, accounts.Present(path, kind, ""), nil
	}
	if len(entries) >= (CategoryCatalog{}).maximumEntries() {
		return nil, nil, &CategoryRefusal{Message: "A mailbox cannot contain more than 200 custom categories"}
	}
	updated := TextValue{}.Copy(entries)
	updated[path] = kind
	catalog[domain] = updated
	return catalog, accounts.Present(path, kind, ""), nil
}
