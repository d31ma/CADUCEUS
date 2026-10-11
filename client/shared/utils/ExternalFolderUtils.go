import (
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// ExternalFolderUtils projects a connected account's flat provider folder
// list into the sidebar's parent and child rows. A parent is inferred only
// when that exact provider folder exists, so an orphan path stays a root and
// never hides behind a synthetic folder.
type ExternalFolderUtils struct{}

// Normalize keeps the bounded, safe provider folders of one account and
// orders them depth first.
func (ExternalFolderUtils) Normalize(raw []ExternalFolder, accountID string) []ExternalFolder {
	folders := ExternalFolderUtils{}.cleaned(raw[:min(250, len(raw))], accountID)
	return ExternalFolderUtils{}.ordered(ExternalFolderUtils{}.parented(folders, ExternalFolderUtils{}.byName(folders)))
}

// Visible keeps the rows whose whole ancestor chain is expanded, and marks
// each row for the view.
func (ExternalFolderUtils) Visible(folders []ExternalFolder, expanded map[string]bool, account ExternalAccount) []ExternalFolder {
	return ExternalFolderUtils{}.shownRows(folders, expanded, ExternalFolderUtils{}.writable(account))
}

// OpenKey is the expanded-folder key of one folder of one account; two
// accounts with the same provider path never collide.
func (ExternalFolderUtils) OpenKey(accountID string, stableID string) string {
	return url.QueryEscape(accountID) + ":" + url.QueryEscape(stableID)
}

// Writable is true when the account can change its folders now.
func (ExternalFolderUtils) Writable(account ExternalAccount) bool {
	return ExternalFolderUtils{}.writable(account)
}

func (ExternalFolderUtils) writable(account ExternalAccount) bool {
	return account.Capabilities.MailWrite && !slices.Contains([]string{"paused", "auth_required", "deleting"}, account.State)
}

func (ExternalFolderUtils) cleaned(raw []ExternalFolder, accountID string) []ExternalFolder {
	trimmed := []ExternalFolder{}
	for _, candidate := range raw {
		trimmed = append(trimmed, ExternalFolderUtils{}.trimmed(candidate, accountID))
	}
	return ExternalFolderUtils{}.unique(slices.DeleteFunc(trimmed, FolderRuleUtils{}.Unsafe))
}

func (ExternalFolderUtils) trimmed(folder ExternalFolder, accountID string) ExternalFolder {
	folder.StableID = strings.TrimSpace(folder.StableID)
	folder.Revision = strings.TrimSpace(folder.Revision)
	folder.Name = strings.TrimSpace(folder.Name)
	folder.Role = strings.ToLower(strings.TrimSpace(folder.Role))
	folder.Delimiter = strings.TrimSpace(folder.Delimiter)
	folder.Attributes = FolderRuleUtils{}.Attributes(folder.Attributes)
	folder.Folder = TextUtils{}.FirstOf(folder.Role, strings.ToLower(folder.Name))
	folder.AccountID = accountID
	folder.Selectable = !slices.ContainsFunc(folder.Attributes, FolderRuleUtils{}.HoldsNoMail)
	folder.DisplayName = folder.Name
	return folder
}

// unique keeps the first folder of each folder name.
func (ExternalFolderUtils) unique(folders []ExternalFolder) []ExternalFolder {
	first := ExternalFolderUtils{}.firsts(folders)
	return slices.DeleteFunc(slices.Clone(folders), func(folder ExternalFolder) bool { return first[folder.Folder] != folder.StableID })
}

func (ExternalFolderUtils) firsts(folders []ExternalFolder) map[string]string {
	first := map[string]string{}
	for _, folder := range folders {
		first[folder.Folder] = TextUtils{}.FirstOf(first[folder.Folder], folder.StableID)
	}
	return first
}

func (ExternalFolderUtils) byName(folders []ExternalFolder) map[string]ExternalFolder {
	byName := map[string]ExternalFolder{}
	for _, folder := range folders {
		byName[folder.Name] = folder
	}
	return byName
}

// parented links each folder to the nearest existing ancestor by name.
func (ExternalFolderUtils) parented(folders []ExternalFolder, byName map[string]ExternalFolder) []ExternalFolder {
	for index := range folders {
		folders[index] = FolderRuleUtils{}.Linked(folders[index], byName)
	}
	return folders
}

// ordered visits the roots and then each folder's children, system roles
// first and then by name.
func (ExternalFolderUtils) ordered(folders []ExternalFolder) []ExternalFolder {
	children := map[string][]ExternalFolder{}
	for _, folder := range folders {
		children[folder.ParentStableID] = append(children[folder.ParentStableID], folder)
	}
	return ExternalFolderUtils{}.visit(children, "", 0)
}

func (ExternalFolderUtils) visit(children map[string][]ExternalFolder, parent string, depth int) []ExternalFolder {
	result := []ExternalFolder{}
	for _, folder := range (ExternalFolderUtils{}).sorted(children[parent]) {
		folder.Depth = depth
		folder.HasChildren = len(children[folder.StableID]) > 0
		result = append(append(result, folder), ExternalFolderUtils{}.visit(children, folder.StableID, depth+1)...)
	}
	return result
}

func (ExternalFolderUtils) sorted(folders []ExternalFolder) []ExternalFolder {
	sort.SliceStable(folders, func(left int, right int) bool { return FolderRuleUtils{}.Before(folders[left], folders[right]) })
	return folders
}

func (ExternalFolderUtils) shownRows(folders []ExternalFolder, expanded map[string]bool, writable bool) []ExternalFolder {
	shown := map[string]bool{"": true}
	rows := []ExternalFolder{}
	for _, folder := range folders {
		parentExpanded := FolderRuleUtils{}.ParentExpanded(folder.ParentStableID == "", expanded[ExternalFolderUtils{}.OpenKey(folder.AccountID, folder.ParentStableID)])
		visible := FolderRuleUtils{}.RowVisible(shown[folder.ParentStableID], parentExpanded)
		shown[folder.StableID] = visible
		rows = append(rows, map[bool][]ExternalFolder{true: {ExternalFolderUtils{}.decorated(folder, expanded, writable)}}[visible]...)
	}
	return rows
}

func (ExternalFolderUtils) decorated(folder ExternalFolder, expanded map[string]bool, writable bool) ExternalFolder {
	folder.Open = expanded[ExternalFolderUtils{}.OpenKey(folder.AccountID, folder.StableID)]
	folder.RowStyle = "--external-folder-depth:" + strconv.Itoa(min(8, max(0, folder.Depth)))
	folder.Href = FolderRuleUtils{}.Href(folder.Folder, "account="+url.QueryEscape(folder.AccountID))
	folder.Actions = FolderRuleUtils{}.Actions(folder, writable)
	folder.HasActions = folder.Actions != FolderActions{}
	return folder
}

