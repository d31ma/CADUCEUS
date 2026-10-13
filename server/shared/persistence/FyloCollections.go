// FyloCollections names the FYLO collections and buckets CADUCEUS stores
// records in. The names are shared with every earlier version of CADUCEUS.
type FyloCollections struct {
	Domains string
	DomainMigrations string
	Emails string
	Attachments string
	RawMessages string
	Suppressed string
	Users string
	InboxRules string
	PushSubscriptions string
	RateLimits string
	Outbox string
	SendRequests string
	Snoozed string
	MailActions string
	Scheduled string
	FollowUpReminders string
	Templates string
	Signatures string
	CalendarEvents string
	Presence string
	Contacts string
	ContactGroups string
	ThreadShares string
	McpAccessTokens string
	McpAuditEvents string
	Extensions string
	AttachmentFiles string
	RawMessageFiles string
}

// Named answers every collection name.
func (FyloCollections) Named() FyloCollections {
	return FyloCollections{
		Domains: "domains",
		DomainMigrations: "domain-migrations",
		Emails: "emails",
		Attachments: "attachments",
		RawMessages: "raw-messages",
		Suppressed: "suppressed",
		Users: "users",
		InboxRules: "inbox-rules",
		PushSubscriptions: "push-subscriptions",
		RateLimits: "rate-limits",
		Outbox: "outbox",
		SendRequests: "send-requests",
		Snoozed: "snoozed",
		MailActions: "mail-actions",
		Scheduled: "scheduled",
		FollowUpReminders: "follow-up-reminders",
		Templates: "templates",
		Signatures: "signatures",
		CalendarEvents: "calendar-events",
		Presence: "presence",
		Contacts: "contacts",
		ContactGroups: "contact-groups",
		ThreadShares: "thread-shares",
		McpAccessTokens: "mcp-access-tokens",
		McpAuditEvents: "mcp-audit-events",
		Extensions: "extensions",
		AttachmentFiles: "attachment-files",
		RawMessageFiles: "raw-message-files",
	}
}
