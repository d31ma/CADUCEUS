// MailboxListTestSetup answers the mailbox list's requests in test units with
// fixed server answers, keyed by path.
// @TestSetup
type MailboxListTestSetup struct{}

func (MailboxListTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	config := NewBehaviorConfiguration()
	for _, name := range []string{"inbox", "threads", "search", "reminders", "send", "activity", "categories", "contacts"} {
		config.Http[name] = MailboxListTestSetup{}
	}
	return config, nil
}

func (MailboxListTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	email := func(id string, folder string, read bool) map[string]any {
		return map[string]any{"id": id, "domain": "example.com", "folder": folder, "systemFolder": folder, "sender": "grace@example.net", "subject": "Subject " + id, "read": read, "receivedAt": "2026-10-11T13:05:00Z", "labels": []any{}}
	}
	answers := map[string]BehaviorValue{
		"/inbox":                []any{email("one", "inbox", false), email("two", "inbox", true), email("three", "archive", true)},
		"/threads":              []any{map[string]any{"id": "t1", "subject": "Thread", "personalLevel": 2, "emails": []any{email("one", "inbox", false), email("two", "inbox", true)}}},
		"/search":               []any{email("one", "inbox", false)},
		"/reminders":            []any{map[string]any{"id": "r1", "emailId": "e1", "subject": "Follow up", "recipient": "x@example.com", "remindAt": "2026-10-13T09:00:00Z", "status": "scheduled"}},
		"/send/scheduled":       []any{map[string]any{"id": "s1", "subject": "Later", "to": []any{"a@example.com", "b@example.com"}, "sendAt": "2026-10-13T09:00:00Z", "status": "queued", "attempts": 0}},
		"/send/scheduled/s1":    map[string]any{"id": "s1", "subject": "Later", "to": []any{"a@example.com"}, "replyTo": []any{}, "html": "<p>Hi</p>", "sendAt": "2026-10-13T09:00:00Z"},
		"/activity":             map[string]any{"events": []any{map[string]any{"id": "v1", "type": "click", "messageId": "m1", "subject": "Hello", "recipient": "", "url": "https://example.com/a", "timestamp": "2026-10-11T13:05:00Z"}}, "metrics": map[string]any{"sent": 4, "received": 2, "trackingRate": 33.333, "openRate": 100, "daily": []any{map[string]any{"date": "2026-10-10", "sent": 2, "received": 1}}, "bySubject": []any{map[string]any{"subject": "Hello", "sent": 2, "opened": 1}}}, "generatedAt": "2026-10-11T13:05:00Z"},
		"/categories":           map[string]any{"accounts": []any{map[string]any{"domain": "example.com", "capabilities": map[string]any{"folders": true, "labels": true, "create": true}, "categories": []any{map[string]any{"path": "archive", "kind": "folder", "displayName": "Archive"}, map[string]any{"path": "work", "kind": "label", "displayName": "Work"}}}}},
		"/contacts/suggestions": []any{map[string]any{"email": "grace@example.net", "name": "Grace Hopper"}},
	}
	return answers[request.Path], nil
}
