import "net/http"

// TestResetRoute answers DELETE /test/reset: it wipes every FYLO collection.
// It answers only when the deployment enables the test routes.
type TestResetRoute struct {
	Fylo *FyloHelper
}

// Reset deletes every document of every collection and bucket.
func (route TestResetRoute) Reset(request *http.Request) *http.Response {
	if _, refusal := (RequestBody{}).Value(request); refusal != nil {
		return refusal
	}
	if !(Security{}).TestRoutesEnabled() {
		return Answer{}.Forbidden("Only available in test mode")
	}
	database := route.Fylo.Database()
	for _, name := range route.collections() {
		collection := FyloCollection{Database: database, Name: name}
		records, err := collection.All()
		if err != nil {
			return RouteAccess{}.Failure(err)
		}
		for _, record := range records {
			if err := collection.Delete(record.Id); err != nil {
				return RouteAccess{}.Failure(err)
			}
		}
	}
	return Answer{}.Ok(map[string]any{"reset": true})
}

// collections answers every collection and bucket the former server named,
// including the authentication collections it once kept in FYLO.
func (TestResetRoute) collections() []string {
	named := FyloCollections{}.Named()
	return []string{
		named.Domains, named.DomainMigrations, named.Emails, named.Attachments, named.RawMessages,
		named.Suppressed, named.Users, named.InboxRules, "mfa-devices", "mfa-sessions", "setup-sessions",
		"auth-credentials", "auth-transactions", "recovery-codes", "auth-audit-events",
		named.PushSubscriptions, named.RateLimits, named.Outbox, named.SendRequests, named.Snoozed,
		named.MailActions, named.Scheduled, named.FollowUpReminders, named.Templates, named.Signatures,
		named.CalendarEvents, named.Presence, named.Contacts, named.ContactGroups, named.ThreadShares,
		named.McpAccessTokens, named.McpAuditEvents, named.Extensions, named.AttachmentFiles, named.RawMessageFiles,
	}
}
