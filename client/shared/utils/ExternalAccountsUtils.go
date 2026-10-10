import (
	"regexp"
	"slices"
	"sort"
	"strings"
)

// ExternalAccountsUtils keeps the connected accounts that the composer may
// offer as From identities, like externalSenderIdentities in
// client/shared/scripts/external-accounts-ui.js.
type ExternalAccountsUtils struct{}

// SenderIdentities lists each sendable connected mailbox once.
func (ExternalAccountsUtils) SenderIdentities(accounts []ExternalAccount) []SenderIdentity {
	listed := ExternalAccountsUtils{}.presented(ExternalAccountsUtils{}.valid(accounts[:min(len(accounts), ExternalAccountsUtils{}.mostAccounts())]))
	ExternalAccountsUtils{}.sortByOrder(listed)
	return ExternalAccountsUtils{}.identities(listed, map[string]bool{})
}

func (ExternalAccountsUtils) mostAccounts() int { return 50 }

func (ExternalAccountsUtils) valid(accounts []ExternalAccount) []ExternalAccount {
	kept := []ExternalAccount{}
	for _, account := range accounts {
		kept = append(kept, ExternalAccountsUtils{}.whenValid(account)...)
	}
	return kept
}

func (ExternalAccountsUtils) whenValid(account ExternalAccount) []ExternalAccount {
	account.Label = strings.TrimSpace(account.Label)
	account.MailboxAddress = strings.TrimSpace(account.MailboxAddress)
	known := ExternalAccountsUtils{}.identifier().MatchString(strings.TrimSpace(account.ID))
	providers := []string{"gmail", "microsoft", "imap"}
	states := []string{"ready", "syncing", "idle", "auth_required", "degraded", "paused", "deleting"}
	if known && account.MailboxAddress != "" && slices.Contains(providers, account.Provider) && slices.Contains(states, account.State) && account.Revision >= 1 && account.Revision == float64(int64(account.Revision)) {
		return []ExternalAccount{account}
	}
	return nil
}

func (ExternalAccountsUtils) presented(accounts []ExternalAccount) []ExternalAccount {
	shown := []ExternalAccount{}
	for _, account := range accounts {
		account.Label = ExternalAccountsUtils{}.label(account)
		account.Color = ExternalAccountsUtils{}.color(account.Color)
		shown = append(shown, account)
	}
	return shown
}

func (ExternalAccountsUtils) label(account ExternalAccount) string {
	if account.Label == "" {
		return account.MailboxAddress
	}
	return account.Label
}

func (ExternalAccountsUtils) color(color string) string {
	if regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`).MatchString(strings.TrimSpace(color)) {
		return strings.TrimSpace(color)
	}
	return "#5f6368"
}

func (ExternalAccountsUtils) identifier() *regexp.Regexp {
	return regexp.MustCompile(`^ext_[A-Za-z0-9_-]{20,80}$`)
}

func (ExternalAccountsUtils) sortByOrder(accounts []ExternalAccount) {
	sort.SliceStable(accounts, func(left int, right int) bool {
		return ExternalAccountsUtils{}.before(accounts[left], accounts[right])
	})
}

func (ExternalAccountsUtils) before(left ExternalAccount, right ExternalAccount) bool {
	if left.Order != right.Order {
		return left.Order < right.Order
	}
	return left.Label < right.Label
}

func (ExternalAccountsUtils) identities(accounts []ExternalAccount, seen map[string]bool) []SenderIdentity {
	identities := []SenderIdentity{}
	for _, account := range accounts {
		identities = append(identities, ExternalAccountsUtils{}.sendable(account, seen)...)
	}
	return identities
}

func (ExternalAccountsUtils) sendable(account ExternalAccount, seen map[string]bool) []SenderIdentity {
	address := strings.ToLower(account.MailboxAddress)
	if !account.Capabilities.Send || !slices.Contains([]string{"ready", "syncing", "idle", "degraded"}, account.State) || seen[address] {
		return nil
	}
	seen[address] = true
	return []SenderIdentity{{Address: address, Label: account.Label, AccountID: account.ID, Provider: account.Provider, Color: account.Color}}
}
