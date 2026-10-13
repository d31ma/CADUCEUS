import "strings"

// ComposeTestSetup answers the composer's requests in test units with fixed
// server answers, keyed by method and path. A PUT of draft d2 answers the
// conflict of a stale version, and any other request is refused.
// @TestSetup
type ComposeTestSetup struct{}

func (ComposeTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	config := NewBehaviorConfiguration()
	for _, name := range []string{"drafts", "send", "contacts"} {
		config.Http[name] = ComposeTestSetup{}
	}
	return config, nil
}

func (ComposeTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	draft := map[string]any{"id": "d1", "sender": "owner@example.com", "to": []any{"ada@example.com"}, "replyTo": []any{"desk@example.com"}, "subject": "Saved subject", "body": "Saved text", "htmlBody": "<p>Saved text</p>", "draftClientId": "client-1", "draftVersion": 3, "attachments": []any{}}
	answers := map[string]BehaviorValue{
		"GET /drafts/d1":            draft,
		"POST /drafts":              map[string]any{"id": "d9", "draftClientId": "client-9", "draftVersion": 1},
		"PUT /drafts/d1":            map[string]any{"id": "d1", "draftClientId": "client-1", "draftVersion": 4},
		"PUT /drafts/d2":            map[string]any{"error": "Draft changed in another session", "conflict": true, "currentVersion": 5},
		"DELETE /drafts/d1":         map[string]any{"discarded": "d1"},
		"POST /send":                map[string]any{"sent": true, "queued": true, "undoId": "undo-1"},
		"GET /contacts/suggestions": []any{map[string]any{"email": "grace@example.net", "name": "Grace Hopper"}, map[string]any{"email": "ops@example.net", "source": "mail"}},
	}
	key := request.Method + " " + strings.TrimSuffix(request.Path, "/")
	answer, found := answers[key]
	if !found {
		return nil, &BehaviorError{Code: "HTTP_STATUS", Message: "The browser HTTP request failed.", Path: key}
	}
	return answer, nil
}
