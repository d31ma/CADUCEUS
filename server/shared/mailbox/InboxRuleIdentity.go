// InboxRuleIdentity is what the authenticated boundary, never the request,
// says about a rule: its id (none for a preview), owner, mailbox and order.
type InboxRuleIdentity struct {
	Id         string
	OwnerEmail string
	Mailbox    string
	Order      any
}
