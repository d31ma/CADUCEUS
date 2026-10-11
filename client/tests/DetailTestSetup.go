// DetailTestSetup answers the message reader's requests in test units with
// fixed server answers, keyed by method and path.
// @TestSetup
type DetailTestSetup struct{}

func (DetailTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	config := NewBehaviorConfiguration()
	for _, name := range []string{"inbox", "users", "threads", "contacts", "search", "github", "snooze", "calendar", "threadShares", "threadshares"} {
		config.Http[name] = DetailTestSetup{}
	}
	return config, nil
}

func (DetailTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	message := func(id string, receivedAt string) map[string]any {
		return map[string]any{
			"id": id, "domain": "example.com", "folder": "inbox", "systemFolder": "inbox", "sender": "grace@example.net", "headerFrom": "Grace Hopper <grace@example.net>",
			"recipient": "owner@example.com", "to": []any{"owner@example.com"}, "cc": []any{"team@example.com"}, "subject": "Plans " + id, "read": false, "receivedAt": receivedAt,
			"rfcMessageId": "<" + id + "@example.net>", "references": []any{"<root@example.net>"}, "labels": []any{},
			"body": "Hello there\n\nOn Monday, Ada wrote:\n> earlier text", "htmlBody": `<p>Hello <img src="https://images.example/photo.png" width="200" alt="Photo"><a href="https://example.org/a">example.org</a></p>`,
			"hasRawSource": true, "attachments": []any{map[string]any{"id": "a1", "filename": "notes.txt", "size": 2048, "contentType": "text/plain"}},
			"calendarInvite": map[string]any{"id": "ev1", "title": "Sync", "start": "2026-10-12T14:00:00Z", "end": "2026-10-12T15:00:00Z", "canRespond": true, "status": "confirmed", "myStatus": "needsAction"},
		}
	}
	answers := map[string]BehaviorValue{
		"GET /inbox/m1":                  map[string]any(message("m1", "2026-10-11T13:05:00Z")),
		"PUT /inbox/m1":                  map[string]any{"id": "m1", "read": true},
		"DELETE /inbox/m1":               map[string]any{"ok": true},
		"GET /inbox/m1/source":           map[string]any{"filename": "m1.eml", "contentBase64": "U3ViamVjdDogUGxhbnMNCg0KSGVsbG8="},
		"GET /inbox/m1/attachments/a1":   map[string]any{"id": "a1", "filename": "notes.txt", "contentType": "text/plain", "contentBase64": "aGVsbG8gbm90ZXM="},
		"POST /inbox/m1/images":          map[string]any{"images": []any{map[string]any{"url": "https://images.example/photo.png", "contentType": "image/gif", "contentBase64": "R0lGODlhAQABAAAAACw=", "size": 14}}},
		"GET /users/me/preferences":      map[string]any{"remoteImagePolicy": "block", "trustedImageSenders": []any{}},
		"PUT /users/me/preferences":      map[string]any{"trustedImageSenders": []any{"grace@example.net"}},
		"POST /send":                     map[string]any{"id": "sent1"},
		"GET /threads":                   []any{map[string]any{"subject": "Plans", "emails": []any{message("m2", "2026-10-11T15:00:00Z"), message("m1", "2026-10-11T13:05:00Z")}}},
		"GET /contacts":                  []any{map[string]any{"name": "Grace Hopper", "title": "Admiral", "emails": []any{map[string]any{"value": "grace@example.net"}}}},
		"GET /search":                    []any{map[string]any{"id": "r1", "subject": "Earlier", "snippet": "  An   earlier  note ", "sender": "grace@example.net", "receivedAt": "2026-10-01T09:00:00Z", "threadId": "t9"}},
		"GET /github/profile":            map[string]any{"status": "found", "profile": map[string]any{"login": "grace", "profileUrl": "https://github.com/grace", "repositories": []any{}}},
		"POST /snooze":                   map[string]any{"snoozeId": "z1", "count": 1},
		"DELETE /snooze/z1":              map[string]any{"ok": true},
		"POST /calendar/invites/respond": map[string]any{"ok": true},
		"GET /thread-shares":             []any{map[string]any{"id": "s1", "messageCount": 2, "active": true, "expiresAt": "2099-01-01T00:00:00Z"}, map[string]any{"id": "s0", "messageCount": 1, "active": false, "revokedAt": "2026-10-01T09:00:00Z", "expiresAt": "2026-10-02T09:00:00Z"}},
		"POST /thread-shares":            map[string]any{"id": "s2", "url": "/share/secret", "expiresAt": "2099-01-01T00:00:00Z"},
		"DELETE /thread-shares/s1":       map[string]any{"ok": true},
	}
	return answers[request.Method+" "+request.Path], nil
}
