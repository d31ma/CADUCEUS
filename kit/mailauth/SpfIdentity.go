package mailauth

import "strings"

type spfIdentity struct {
	local  string
	domain string
	helo   string
	sender string
}

func newSpfIdentity(input Input) spfIdentity {
	identity := spfIdentity{helo: input.Helo}
	sender := identity.envelope(strings.Trim(strings.TrimSpace(input.MailFrom), "<>"))
	identity.local, identity.domain = identity.split(sender)
	identity.sender = identity.local + "@" + identity.domain
	return identity
}

func (identity spfIdentity) envelope(mailFrom string) string {
	if mailFrom == "" {
		return identity.postmaster() + "@" + identity.helo
	}
	return mailFrom
}

func (identity spfIdentity) split(sender string) (string, string) {
	at := strings.LastIndex(sender, "@")
	if at < 0 {
		return identity.postmaster(), identity.clean(sender)
	}
	return sender[:at], identity.clean(sender[at+1:])
}

func (spfIdentity) clean(domain string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
}

func (spfIdentity) postmaster() string {
	return "postmaster"
}
