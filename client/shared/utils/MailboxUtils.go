import (
	"cmp"
	"slices"
)

// MailboxUtils knows the hosted mailbox domains of a session and the personal
// display names the account gave them in its preferences.
type MailboxUtils struct{}

// Domains are the session's mailbox domains, lower case, once each, sorted.
func (MailboxUtils) Domains(session AccountSession) []string {
	domains := PreferenceUtils{}.Unique(session.Domains)
	slices.Sort(domains)
	return domains
}

func (MailboxUtils) Names(values map[string]any) map[string]string {
	names := map[string]string{}
	PreferenceUtils{}.Decode(values["mailboxNames"], &names)
	return names
}

// Label is the personal name of a mailbox, or its domain.
func (MailboxUtils) Label(names map[string]string, domain string) string {
	lower := PreferenceUtils{}.Lower(domain)
	return cmp.Or(names[lower], lower)
}
