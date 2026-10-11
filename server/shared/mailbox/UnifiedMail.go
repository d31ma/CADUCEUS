import (
	"errors"
	"math"
	"strings"
)

// UnifiedMail reads and changes connected-account mail only through the
// owner-authorized operations of the external account boundary, and maps it
// onto the hosted mailbox contract.
type UnifiedMail struct {
	Accounts ProviderMailbox
}

// Summaries answers the caller's provider rows as inbox rows (at most 1000).
func (unified UnifiedMail) Summaries(token string, filter SummaryFilter) ([]map[string]any, error) {
	input := map[string]any{
		"accountId": filter.AccountId, "folderRole": filter.FolderRole, "folderStableId": filter.FolderStableId,
		"query": filter.Query, "limit": unified.bounded(filter.Limit), "offset": math.Max(0, float64(filter.Offset)),
	}
	if filter.Read != nil {
		input["read"] = *filter.Read
	}
	if filter.Starred != nil {
		input["starred"] = *filter.Starred
	}
	if filter.ReceivedAtFrom != nil {
		input["receivedAtFrom"] = *filter.ReceivedAtFrom
	}
	if filter.OldestFirst {
		input["oldestFirst"] = true
	}
	response, err := unified.Accounts.UnifiedMessages(token, input)
	if err != nil {
		return nil, err
	}
	summaries := []map[string]any{}
	for _, row := range (TextValue{}).List(response["messages"]) {
		fields, _ := row.(map[string]any)
		if summary := (ExternalMessage{}).Summary(fields); summary != nil {
			summaries = append(summaries, summary)
		}
	}
	return summaries, nil
}

func (UnifiedMail) bounded(limit int) int {
	if limit <= 0 {
		return 1000
	}
	return min(1000, limit)
}

// Load answers one provider message with its account, folders, enriched
// index row and (with source) its RFC 822 source, or nil when the identity
// is not valid or the message is gone.
func (unified UnifiedMail) Load(token string, routeId string, withSource bool) (*ExternalLoaded, error) {
	accountId, stableId, valid := ExternalMessage{}.ParseId(routeId)
	if !valid {
		return nil, nil
	}
	account, err := unified.Accounts.Get(token, accountId)
	if err != nil {
		return nil, err
	}
	folderResponse, err := unified.Accounts.Folders(token, accountId)
	if err != nil {
		return nil, err
	}
	var source map[string]any
	if withSource {
		source, err = unified.Accounts.MessageSource(token, accountId, stableId)
	} else {
		source, err = unified.Accounts.Message(token, accountId, stableId)
	}
	if err != nil {
		return nil, err
	}
	message, _ := source["message"].(map[string]any)
	_, hasSource := source["rawRFC822Base64"].(string)
	if !(TextValue{}).Truthy(account["id"]) || message == nil || (withSource && !hasSource) {
		return nil, nil
	}
	folders := TextValue{}.List(folderResponse["folders"])
	loaded := &ExternalLoaded{AccountId: accountId, StableId: stableId, Account: account, Folders: folders, RawBase64: source["rawRFC822Base64"]}
	loaded.Indexed = unified.indexed(message, account, folders)
	return loaded, nil
}

func (unified UnifiedMail) indexed(message map[string]any, account map[string]any, folders []any) map[string]any {
	text := TextValue{}
	indexed := text.Copy(message)
	var folder map[string]any
	for _, candidate := range folders {
		if fields, isObject := candidate.(map[string]any); isObject && fields["stableId"] == message["folderStableId"] {
			folder = fields
			break
		}
	}
	indexed["accountId"] = account["id"]
	indexed["mailboxAddress"] = account["mailboxAddress"]
	indexed["accountLabel"] = account["label"]
	indexed["accountColor"] = account["color"]
	indexed["folderName"] = text.Or(folder["name"], message["folderStableId"])
	indexed["folderRole"] = text.Or(folder["role"], unified.inferredRole(text.Or(folder["name"], message["folderStableId"])))
	return indexed
}

func (UnifiedMail) inferredRole(value any) string {
	folder := strings.ToLower(TextValue{}.Trim(TextValue{}.Of(value)))
	if (CategoryMappings{}).IsRole(folder) {
		return folder
	}
	return ""
}

// ResolveFolder answers the provider folder a mailbox folder name means: by
// role first, then by stable id or name; nil when there is none.
func (UnifiedMail) ResolveFolder(folders []any, requested string) map[string]any {
	text := TextValue{}
	wanted := strings.ToLower(text.Trim(requested))
	if wanted == "" {
		return nil
	}
	for _, candidate := range folders {
		if fields, _ := candidate.(map[string]any); fields != nil && strings.ToLower(text.Trim(text.Of(text.Or(fields["role"], "")))) == wanted {
			return fields
		}
	}
	for _, candidate := range folders {
		fields, _ := candidate.(map[string]any)
		for _, value := range []any{fields["stableId"], fields["name"]} {
			if fields != nil && strings.ToLower(text.Trim(text.Of(text.Or(value, "")))) == wanted {
				return fields
			}
		}
	}
	return nil
}

// QueuePatch persists one provider mutation per changed property (read,
// starred, then folder) and answers them. A folder the account does not
// expose is an ExternalFolderError.
func (unified UnifiedMail) QueuePatch(token string, loaded *ExternalLoaded, patch map[string]any, idempotencyPrefix string) ([]map[string]any, error) {
	if loaded == nil {
		return []map[string]any{}, nil
	}
	type request struct {
		kind   string
		value  bool
		target string
	}
	requests := []request{}
	if read, isFlag := patch["read"].(bool); isFlag {
		requests = append(requests, request{kind: "set_read", value: read})
	}
	if starred, isFlag := patch["starred"].(bool); isFlag {
		requests = append(requests, request{kind: "set_starred", value: starred})
	}
	if folder, isText := patch["folder"].(string); isText {
		target := unified.ResolveFolder(loaded.Folders, folder)
		if target == nil {
			return nil, &ExternalFolderError{Folder: folder}
		}
		requests = append(requests, request{kind: "move", target: TextValue{}.Of(target["stableId"])})
	}
	mutations := []map[string]any{}
	for _, item := range requests {
		mutation, err := unified.enqueue(token, loaded, map[string]any{
			"idempotencyKey": unified.idempotencyKey(idempotencyPrefix, item.kind), "kind": item.kind,
			"messageStableIds": []string{loaded.StableId}, "targetFolderId": item.target, "value": item.value,
		})
		if err != nil {
			return nil, err
		}
		mutations = append(mutations, mutation)
	}
	return mutations, nil
}

func (UnifiedMail) idempotencyKey(prefix string, kind string) string {
	stable := TextValue{}.Trim(prefix)
	if stable == "" {
		return "mailbox-" + kind + "-" + Security{}.RandomUuid()
	}
	return TextValue{}.Prefix(stable+"-"+kind, 128)
}

// QueueDelete queues a provider-side permanent delete of one owned message.
func (unified UnifiedMail) QueueDelete(token string, loaded *ExternalLoaded) (map[string]any, error) {
	return unified.enqueue(token, loaded, map[string]any{
		"idempotencyKey": "mailbox-delete-" + Security{}.RandomUuid(), "kind": "delete",
		"messageStableIds": []string{loaded.StableId}, "targetFolderId": "", "value": false,
	})
}

// enqueue queues one mutation, refreshing the account revision on an
// optimistic-concurrency conflict (at most five attempts).
func (unified UnifiedMail) enqueue(token string, loaded *ExternalLoaded, request map[string]any) (map[string]any, error) {
	revision := loaded.Account["revision"]
	accountId := TextValue{}.Of(loaded.Account["id"])
	for attempt := 0; attempt < 5; attempt++ {
		input := TextValue{}.Copy(request)
		input["revision"] = revision
		mutation, err := unified.Accounts.Mutate(token, accountId, input)
		if err == nil {
			return mutation, nil
		}
		var failure *BoundaryError
		if !errors.As(err, &failure) || unified.Accounts.Status(err) != 409 || attempt == 4 {
			return nil, err
		}
		refreshed, err := unified.Accounts.Get(token, accountId)
		if err != nil {
			return nil, err
		}
		revision = refreshed["revision"]
	}
	return nil, &BoundaryError{Code: "ECONFLICT", Message: "external account changed; refresh and retry"}
}

// Projected answers the loaded message as it reads once a pending patch
// applies.
func (unified UnifiedMail) Projected(loaded *ExternalLoaded, patch map[string]any) *ExternalLoaded {
	if loaded == nil {
		return nil
	}
	indexed := TextValue{}.Copy(loaded.Indexed)
	flags := append([]any{}, TextValue{}.List(indexed["flags"])...)
	if read, isFlag := patch["read"].(bool); isFlag {
		flags = unified.flag(flags, `\Seen`, read)
	}
	if starred, isFlag := patch["starred"].(bool); isFlag {
		flags = unified.flag(flags, `\Flagged`, starred)
	}
	indexed["flags"] = flags
	if folder, isText := patch["folder"].(string); isText {
		if target := unified.ResolveFolder(loaded.Folders, folder); target != nil {
			indexed["folderStableId"] = target["stableId"]
			indexed["folderName"] = target["name"]
			indexed["folderRole"] = TextValue{}.Or(target["role"], unified.inferredRole(target["name"]))
		}
	}
	projected := *loaded
	projected.Indexed = indexed
	return &projected
}

func (UnifiedMail) flag(flags []any, target string, enabled bool) []any {
	values := []any{}
	for _, flag := range flags {
		if !strings.EqualFold(TextValue{}.Of(flag), target) {
			values = append(values, flag)
		}
	}
	if enabled {
		values = append(values, target)
	}
	return values
}
