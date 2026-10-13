import (
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// FolderRuleUtils holds the per-folder rules of the provider folder tree: which
// provider folders are safe to show, how a folder finds its parent, how the
// rows are ordered, and which folder actions the provider allows.
type FolderRuleUtils struct{}

// Unsafe is true for a provider folder whose fields are empty, too long or
// carry control characters.
func (FolderRuleUtils) Unsafe(folder ExternalFolder) bool {
	return FolderRuleUtils{}.unsafe(folder, FolderRuleUtils{}.bounds())
}

// Attributes keeps at most 50 non-empty attributes of at most 128 characters.
func (FolderRuleUtils) Attributes(values []string) []string {
	return slices.DeleteFunc(FolderRuleUtils{}.trimmed(values[:min(50, len(values))]), FolderRuleUtils{}.unfit)
}

func (FolderRuleUtils) trimmed(values []string) []string {
	trimmed := []string{}
	for _, value := range values {
		trimmed = append(trimmed, strings.TrimSpace(value))
	}
	return trimmed
}

// HoldsNoMail is true for the IMAP attributes of a folder that holds no mail.
func (FolderRuleUtils) HoldsNoMail(attribute string) bool {
	return slices.Contains([]string{"\\noselect", "\\nonexistent"}, strings.ToLower(attribute))
}

// Linked gives a folder its nearest existing ancestor, and the display name
// below it.
func (FolderRuleUtils) Linked(folder ExternalFolder, byName map[string]ExternalFolder) ExternalFolder {
	return FolderRuleUtils{}.linked(folder, FolderRuleUtils{}.parent(folder, byName, FolderRuleUtils{}.prefixes(folder)))
}

// Before orders system roles first, then names without case.
func (FolderRuleUtils) Before(left ExternalFolder, right ExternalFolder) bool {
	return FolderRuleUtils{}.before(FolderRuleUtils{}.rank(left.Role), FolderRuleUtils{}.rank(right.Role), strings.ToLower(left.Name) < strings.ToLower(right.Name))
}

// Href is the mailbox route of a provider folder.
func (FolderRuleUtils) Href(folder string, query string) string {
	inbox := FolderRuleUtils{}.inbox()
	return FolderRuleUtils{}.href(folder == inbox, folder) + "?" + query
}

// Actions are the folder changes the provider allows: protected folders
// (any role, or the inbox) cannot be renamed or deleted.
func (FolderRuleUtils) Actions(folder ExternalFolder, writable bool) FolderActions {
	return FolderRuleUtils{}.actions(folder, writable, FolderRuleUtils{}.either(folder.Role != "", strings.EqualFold(folder.Name, FolderRuleUtils{}.inbox())))
}

func (FolderRuleUtils) unsafe(folder ExternalFolder, bounds FolderBounds) bool {
	return slices.Contains([]bool{
		folder.StableID == "", len(folder.StableID) > bounds.StableID, folder.Name == "", len(folder.Name) > bounds.Name,
		len(folder.Role) > bounds.Role, len(folder.Delimiter) > bounds.Delimiter, folder.Folder == "",
		regexp.MustCompile("[\x00\r\n]").MatchString(folder.StableID + folder.Name + folder.Delimiter),
	}, true)
}

// RowVisible is true for a root row, and for a row whose parent row is shown
// and expanded.
func (FolderRuleUtils) RowVisible(parentShown bool, parentExpanded bool) bool {
	return FolderRuleUtils{}.rowVisible(parentShown, parentExpanded)
}

// ParentExpanded is true for a root row or a row under an expanded parent.
func (FolderRuleUtils) ParentExpanded(root bool, expanded bool) bool {
	return FolderRuleUtils{}.either(root, expanded)
}

func (FolderRuleUtils) rowVisible(parentShown bool, parentExpanded bool) bool {
	return parentShown && parentExpanded
}

func (FolderRuleUtils) either(first bool, second bool) bool {
	return first || second
}

func (FolderRuleUtils) unfit(value string) bool {
	return FolderRuleUtils{}.either(value == "", len(value) > FolderRuleUtils{}.bounds().Attribute)
}

// prefixes are the folder's ancestor paths, deepest first. A folder without a
// delimiter has none: no name holds the NUL that stands in for it.
func (FolderRuleUtils) prefixes(folder ExternalFolder) []string {
	delimiter := TextUtils{}.FirstOf(folder.Delimiter, "\x00")
	parts := strings.Split(folder.Name, delimiter)
	prefixes := []string{}
	for count := len(parts) - 1; count > 0; count-- {
		prefixes = append(prefixes, strings.Join(parts[:count], delimiter))
	}
	return prefixes
}

func (FolderRuleUtils) parent(folder ExternalFolder, byName map[string]ExternalFolder, prefixes []string) ExternalFolder {
	index := slices.IndexFunc(prefixes, func(prefix string) bool {
		return !slices.Contains([]string{"", folder.StableID}, byName[prefix].StableID)
	})
	return byName[append([]string{""}, prefixes...)[index+1]]
}

func (FolderRuleUtils) linked(folder ExternalFolder, parent ExternalFolder) ExternalFolder {
	if parent.StableID == "" {
		return folder
	}
	folder.ParentStableID = parent.StableID
	folder.DisplayName = TextUtils{}.FirstOf(folder.Name[min(len(folder.Name), len(parent.Name)+len(folder.Delimiter)):], folder.Name)
	return folder
}

func (FolderRuleUtils) rank(role string) int {
	order := []string{FolderRuleUtils{}.inbox(), "important", "drafts", "sent", "archive", "all", "spam", "trash"}
	return map[bool]int{true: slices.Index(order, role), false: len(order)}[slices.Contains(order, role)]
}

func (FolderRuleUtils) before(leftRank int, rightRank int, alphabetical bool) bool {
	if leftRank != rightRank {
		return leftRank < rightRank
	}
	return alphabetical
}

func (FolderRuleUtils) href(inbox bool, folder string) string {
	if inbox {
		return "/" + FolderRuleUtils{}.inbox()
	}
	return "/folder/" + url.PathEscape(folder)
}

func (FolderRuleUtils) actions(folder ExternalFolder, writable bool, guarded bool) FolderActions {
	changeable := !slices.Contains([]bool{writable, !guarded}, false)
	return FolderActions{
		CreateChild: !slices.Contains([]bool{writable, folder.Delimiter != "", !slices.ContainsFunc(folder.Attributes, FolderRuleUtils{}.noChildren)}, false),
		Rename:      changeable,
		Delete:      changeable,
		Export:      !slices.Contains([]bool{folder.Selectable, folder.Role != "drafts"}, false),
	}
}

func (FolderRuleUtils) noChildren(attribute string) bool {
	return strings.EqualFold(attribute, "\\noinferiors")
}

func (FolderRuleUtils) bounds() FolderBounds {
	return FolderBounds{StableID: 512, Name: 1024, Role: 60, Delimiter: 4, Attribute: 128}
}

func (FolderRuleUtils) inbox() string { return "inbox" }
