import (
	"sort"
	"strings"
)

// McpFolders is the part of the category-mapping service the MCP tools use:
// the folders and labels of each allowed account, and the folder a system
// role maps to. It is a private copy until the mail area's
// CategoryMappingService is ported.
type McpFolders struct {
	Users  *UsersHelper
	Emails *EmailsHelper
}

// Accounts answers, per allowed domain in order, the physical folders and
// labels the owner has, each folder with the system role it carries; nil
// when the owner has no user record.
func (folders McpFolders) Accounts(ownerEmail string, allowedDomains []string) ([]map[string]any, error) {
	_, user, err := folders.Users.FindByEmail(ownerEmail)
	if err != nil || user == nil {
		return nil, err
	}
	domains, _ := user["domains"].([]string)
	mappings := CategoryMappingRules{}.Normalized(user["categoryMappings"], domains)
	catalog := CategoryCatalog{}.Normalized(user["categoryCatalog"], domains)
	emails, err := folders.Emails.List(allowedDomains, EmailListFilters{Folder: "all", OwnerEmail: ownerEmail})
	if err != nil {
		return nil, err
	}
	sorted := append([]string{}, allowedDomains...)
	sort.Strings(sorted)
	accounts := []map[string]any{}
	for _, domain := range sorted {
		accounts = append(accounts, folders.account(domain, folders.assigned(mappings, domain), catalog, emails))
	}
	return accounts, nil
}

func (folders McpFolders) account(domain string, assigned map[string]any, catalog map[string]any, emails []map[string]any) map[string]any {
	paths := []string{}
	labels := []string{}
	add := func(list []string, path string) []string {
		if path == "" || (PreferenceChoices{}).Includes(list, path) {
			return list
		}
		return append(list, path)
	}
	for _, role := range folders.roles() {
		paths = add(paths, RecordValues{}.Text(assigned[role]))
	}
	for _, email := range emails {
		if email["domain"] != domain {
			continue
		}
		paths = add(paths, CategoryPath{}.Normalized(RecordValues{}.Or(email["folder"], "inbox")))
		list, _ := RecordValues{}.List(email["labels"])
		for _, label := range list {
			labels = add(labels, TextValue{}.Of(label))
		}
	}
	entries, _ := catalog[domain].(map[string]any)
	for _, path := range (PreferenceChoices{}).Keys(entries) {
		if entries[path] == "label" {
			labels = add(labels, path)
		} else {
			paths = add(paths, path)
		}
	}
	roles := map[string]string{}
	for _, role := range folders.roles() {
		roles[RecordValues{}.Text(assigned[role])] = role
	}
	categories := []map[string]any{}
	for _, path := range paths {
		categories = append(categories, map[string]any{"path": path, "kind": "folder", "role": roles[path]})
	}
	for _, path := range labels {
		categories = append(categories, map[string]any{"path": path, "kind": "label"})
	}
	sort.SliceStable(categories, func(left, right int) bool {
		order := TextOrder{}
		if kinds := order.Compare(categories[left]["kind"].(string), categories[right]["kind"].(string)); kinds != 0 {
			return kinds < 0
		}
		return order.Compare(categories[left]["path"].(string), categories[right]["path"].(string)) < 0
	})
	return map[string]any{"domain": domain, "categories": categories}
}

// PathFor answers the folder a requested system role maps to in the
// owner's domain, or the requested path itself ("" when it is unsafe).
func (folders McpFolders) PathFor(ownerEmail string, domain string, requested string) (string, error) {
	normalized := CategoryPath{}.Normalized(requested)
	if normalized == "" || !(PreferenceChoices{}).Includes(folders.roles(), normalized) {
		return normalized, nil
	}
	_, user, err := folders.Users.FindByEmail(ownerEmail)
	if err != nil || user == nil {
		return normalized, err
	}
	domains, _ := user["domains"].([]string)
	mapping, _ := CategoryMappingRules{}.Normalized(user["categoryMappings"], domains)[domain].(map[string]any)
	if path := (RecordValues{}).Text(mapping[normalized]); path != "" {
		return path, nil
	}
	return normalized, nil
}

func (folders McpFolders) assigned(mappings map[string]any, domain string) map[string]any {
	if mapping, isObject := mappings[domain].(map[string]any); isObject {
		return mapping
	}
	defaults := map[string]any{}
	for _, role := range folders.roles() {
		defaults[role] = role
	}
	return defaults
}

func (McpFolders) roles() []string {
	return strings.Fields("inbox sent drafts spam archive trash")
}
