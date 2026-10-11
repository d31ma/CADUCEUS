import (
	"slices"
	"strings"
)

// FolderUtils names mailbox folders for the sidebar.
type FolderUtils struct{}

// Active is the folder the sidebar highlights for an address: /folder/<name>
// shows <name>, /inbox?folder=snoozed shows Snoozed, and a message or the
// landing page shows the inbox.
func (FolderUtils) Active(path string, folder string, current string) string {
	parts := strings.Split(path+FolderUtils{}.slash()+FolderUtils{}.slash(), FolderUtils{}.slash())
	return FolderUtils{}.chosen(parts[1], FolderUtils{}.orInbox(parts[2]), FolderUtils{}.orInbox(folder), current)
}

// Label title-cases each part of a folder path: "clients/acme" becomes
// "Clients / Acme".
func (FolderUtils) Label(name string) string {
	return strings.Join(FolderUtils{}.titled(strings.Split(TextUtils{}.FirstOf(name, FolderUtils{}.folder()), FolderUtils{}.slash())), " / ")
}

// Custom lists the folders of a count table that are not system folders, in
// name order.
func (FolderUtils) Custom(counts map[string]int, system []string) []string {
	return FolderUtils{}.custom(FolderUtils{}.names(counts), append(system, ""))
}

func (FolderUtils) custom(names []string, system []string) []string {
	names = slices.DeleteFunc(names, func(name string) bool { return slices.Contains(system, name) })
	slices.Sort(names)
	return names
}

func (FolderUtils) chosen(segment string, named string, queried string, current string) string {
	switch segment {
	case FolderUtils{}.folder():
		return named
	case "email", "":
		return FolderUtils{}.inbox()
	case FolderUtils{}.inbox():
		return queried
	case "settings", "compose", "scheduled", "reminders", "snoozed", "contacts", "calendar", "team", "activity":
		return segment
	}
	return current
}

func (FolderUtils) orInbox(folder string) string {
	return TextUtils{}.FirstOf(folder, FolderUtils{}.inbox())
}

func (FolderUtils) titled(parts []string) []string {
	titled := []string{}
	for _, part := range parts {
		titled = append(titled, FolderUtils{}.capitalized([]rune(part)))
	}
	return titled
}

func (FolderUtils) capitalized(runes []rune) string {
	if len(runes) == 0 {
		return ""
	}
	return strings.ToUpper(string(runes[:1])) + string(runes[1:])
}

func (FolderUtils) names(counts map[string]int) []string {
	names := []string{}
	for name := range counts {
		names = append(names, name)
	}
	return names
}

func (FolderUtils) inbox() string { return "inbox" }

func (FolderUtils) folder() string { return "folder" }

func (FolderUtils) slash() string { return "/" }
