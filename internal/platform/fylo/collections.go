package fylo

// DocumentCollections are the FYLO document collections CADUCEUS keeps, in
// the order the JavaScript Fylo.createDb created them (Fylo.Collections).
// This is the one list: the entry point creates these collections at start,
// and the bridge accepts operations on them and nothing else.
//
// The Go server names the collections its routes use in
// server/shared/persistence/FyloCollections.go; server/ is compiled by
// Tachyon as package main and cannot import this package, so a test checks
// that every name the server uses is listed here.
//
// mfa-devices, mfa-sessions, setup-sessions, auth-credentials,
// auth-transactions, recovery-codes and auth-audit-events held the
// authentication records before SESAME owned them. Nothing writes them now,
// but existing data roots have them and the test reset clears them, so they
// stay created and allowed until a migration removes them.
func DocumentCollections() []string {
	return []string{
		"domains", "domain-migrations", "emails", "attachments", "raw-messages", "suppressed", "users",
		"inbox-rules", "mfa-devices", "mfa-sessions", "setup-sessions", "auth-credentials",
		"auth-transactions", "recovery-codes", "auth-audit-events", "push-subscriptions", "rate-limits",
		"outbox", "send-requests", "snoozed", "mail-actions", "scheduled", "follow-up-reminders",
		"templates", "signatures", "calendar-events", "presence", "contacts", "contact-groups",
		"thread-shares", "mcp-access-tokens", "mcp-audit-events", "extensions",
	}
}

// FileCollections are the FYLO file collections (Fylo.Buckets).
func FileCollections() []string {
	return []string{"attachment-files", "raw-message-files"}
}

// AllCollections answers the document collections followed by the file
// collections.
func AllCollections() []string {
	return append(DocumentCollections(), FileCollections()...)
}
