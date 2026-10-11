import (
	"regexp"
	"slices"
	"sort"
	"strings"
)

// ExternalAccountUtils keeps the bounded fields of connected accounts and
// the texts of the provider-folder dialog.
type ExternalAccountUtils struct{}

// Normalize keeps at most 50 well-formed accounts, in their saved order.
func (ExternalAccountUtils) Normalize(accounts []ExternalAccount) []ExternalAccount {
	kept := slices.DeleteFunc(slices.Clone(accounts[:min(50, len(accounts))]), ExternalAccountUtils{}.malformed)
	return ExternalAccountUtils{}.sorted(ExternalAccountUtils{}.cleanedAll(kept))
}

// Arranged marks which accounts can move up or down and create folders.
func (ExternalAccountUtils) Arranged(accounts []ExternalAccount) []ExternalAccount {
	return ExternalAccountUtils{}.arranged(accounts)
}

// Replaced swaps in the server's copies of updated accounts and reorders.
func (ExternalAccountUtils) Replaced(accounts []ExternalAccount, updates []ExternalAccount) []ExternalAccount {
	return ExternalAccountUtils{}.sorted(ExternalAccountUtils{}.replaced(slices.Clone(accounts), updates))
}

// ValidFolderName accepts 1 to 128 characters without outer spaces or line
// breaks.
func (ExternalAccountUtils) ValidFolderName(name string) bool {
	return ExternalAccountUtils{}.validName(name, len([]rune(name)) <= FolderRuleUtils{}.bounds().Attribute)
}

// Mutation is the provider folder change that the dialog queues.
func (ExternalAccountUtils) Mutation(dialog FolderDialog, revision int, key string) FolderMutation {
	mutation := FolderMutation{Revision: revision, IdempotencyKey: "folder-" + dialog.Mode + "-" + key, Kind: "folder_" + dialog.Mode, MessageStableIds: []string{}, FolderName: dialog.Name}
	return ExternalAccountUtils{}.placed(mutation, dialog.Mode, dialog.Folder)
}

// DialogTexts are the title, description and button text of the dialog.
func (ExternalAccountUtils) DialogTexts(dialog FolderDialog, inFolder bool, busy bool) []string {
	modes := ExternalAccountUtils{}.modes()
	return []string{
		ExternalAccountUtils{}.title(dialog.Mode, inFolder, modes),
		ExternalAccountUtils{}.description(dialog.Mode, inFolder, dialog.Name, modes),
		ExternalAccountUtils{}.action(dialog.Mode, busy, modes),
	}
}

func (ExternalAccountUtils) malformed(account ExternalAccount) bool {
	return slices.Contains([]bool{
		strings.HasPrefix(account.ID, "ext_") && TokenUtils{}.Fits(strings.TrimPrefix(account.ID, "ext_"), TokenUtils{}.Identifier()+"_-", TokenBounds{Shortest: 20, Longest: 80}),
		strings.TrimSpace(account.MailboxAddress) != "",
		slices.Contains([]string{"gmail", "microsoft", "imap"}, account.Provider),
		slices.Contains(ExternalAccountUtils{}.states(), account.State),
		account.Revision >= 1,
	}, false)
}

// Readable is true when the sidebar may list the account's folders.
func (ExternalAccountUtils) Readable(account ExternalAccount) bool {
	return ExternalAccountUtils{}.readable(account)
}

func (ExternalAccountUtils) readable(account ExternalAccount) bool {
	return account.Capabilities.MailRead && account.State != ExternalAccountUtils{}.states()[6]
}

func (ExternalAccountUtils) cleanedAll(accounts []ExternalAccount) []ExternalAccount {
	for index := range accounts {
		accounts[index] = ExternalAccountUtils{}.cleaned(accounts[index])
	}
	return accounts
}

func (ExternalAccountUtils) cleaned(account ExternalAccount) ExternalAccount {
	account.MailboxAddress = strings.TrimSpace(account.MailboxAddress)
	account.Label = TextUtils{}.FirstOf(strings.TrimSpace(account.Label), account.MailboxAddress)
	account.Color = ExternalAccountUtils{}.color(strings.TrimSpace(account.Color))
	return account
}

func (ExternalAccountUtils) color(color string) string {
	if regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`).MatchString(color) {
		return color
	}
	return "#5f6368"
}

func (ExternalAccountUtils) arranged(accounts []ExternalAccount) []ExternalAccount {
	for index := range accounts {
		accounts[index].CanMoveUp = index > 0
		accounts[index].CanMoveDown = index < len(accounts)-1
		accounts[index].CanCreateFolder = ExternalFolderUtils{}.Writable(accounts[index])
	}
	return accounts
}

func (ExternalAccountUtils) sorted(accounts []ExternalAccount) []ExternalAccount {
	sort.SliceStable(accounts, func(left int, right int) bool {
		return ExternalAccountUtils{}.before(accounts[left], accounts[right])
	})
	return accounts
}

func (ExternalAccountUtils) before(left ExternalAccount, right ExternalAccount) bool {
	if left.Order != right.Order {
		return left.Order < right.Order
	}
	return left.ID < right.ID
}

func (ExternalAccountUtils) byID(updates []ExternalAccount) map[string]ExternalAccount {
	byID := map[string]ExternalAccount{}
	for _, update := range updates {
		byID[update.ID] = ExternalAccountUtils{}.cleaned(update)
	}
	return byID
}

func (ExternalAccountUtils) replaced(accounts []ExternalAccount, updates []ExternalAccount) []ExternalAccount {
	byID := ExternalAccountUtils{}.byID(updates)
	for index, account := range accounts {
		accounts[index] = map[bool]ExternalAccount{true: byID[account.ID], false: account}[byID[account.ID].ID != ""]
	}
	return accounts
}

func (ExternalAccountUtils) validName(name string, short bool) bool {
	return !slices.Contains([]bool{name != "", name == strings.TrimSpace(name), short, !strings.ContainsAny(name, "\x00\r\n")}, false)
}

func (ExternalAccountUtils) placed(mutation FolderMutation, mode string, folder string) FolderMutation {
	modes := ExternalAccountUtils{}.modes()
	switch mode {
	case modes.Create:
		mutation.ParentStableID = folder
	case modes.Delete:
		mutation.FolderStableID = folder
		mutation.FolderName = ""
	default:
		mutation.FolderStableID = folder
	}
	return mutation
}

func (ExternalAccountUtils) title(mode string, inFolder bool, modes FolderModes) string {
	switch {
	case mode == modes.Rename:
		return "Rename connected folder"
	case mode == modes.Delete:
		return "Delete connected folder"
	case inFolder:
		return "Create subfolder"
	}
	return "Create connected folder"
}

func (ExternalAccountUtils) description(mode string, inFolder bool, folderName string, modes FolderModes) string {
	quoted, stop := "“"+folderName+"”", "."
	switch {
	case mode == modes.Delete:
		return "Delete " + quoted + " from the connected mailbox? The provider may also delete messages it contains."
	case mode == modes.Rename:
		return "Choose a new name for " + quoted + stop
	case inFolder:
		return "Create a folder inside " + quoted + stop
	}
	return "Create a folder at the top level of this connected mailbox."
}

func (ExternalAccountUtils) action(mode string, busy bool, modes FolderModes) string {
	switch {
	case busy:
		return "Working…"
	case mode == modes.Rename:
		return "Rename folder"
	case mode == modes.Delete:
		return "Delete folder"
	}
	return "Create folder"
}

func (ExternalAccountUtils) states() []string {
	return []string{"ready", "syncing", "idle", "auth_required", "degraded", "paused", "deleting"}
}

func (ExternalAccountUtils) modes() FolderModes {
	return FolderModes{Create: "create", Rename: "rename", Delete: "delete"}
}
