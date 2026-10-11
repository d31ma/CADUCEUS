import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

// ExternalSubmission holds the connected-account send identities: the
// opaque "extundo." undo token (re-authorized server-side on use), the
// "extmsg." message id of a provider message, and the per-recipient tracked
// MIME input a provider submission sends.
type ExternalSubmission struct{}

// UndoToken answers the undo token of a delayed provider submission, or ""
// when an identity is malformed.
func (submission ExternalSubmission) UndoToken(accountId string, submissionId string, messageId string, archiveActionId string) string {
	if !submission.accountPattern().MatchString(accountId) || !submission.submissionPattern().MatchString(submissionId) || (Threading{}).NormalizeMessageId(messageId) == "" || (archiveActionId != "" && !submission.actionPattern().MatchString(archiveActionId)) {
		return ""
	}
	payload := map[string]any{"accountId": accountId, "submissionId": submissionId, "messageId": messageId}
	if archiveActionId != "" {
		payload["archiveActionId"] = archiveActionId
	}
	return "extundo." + base64.RawURLEncoding.EncodeToString([]byte(StoredRecords{}.JsonText(payload)))
}

// ParsedUndoToken answers the identities of a valid undo token, or nil.
func (submission ExternalSubmission) ParsedUndoToken(value string) map[string]any {
	decoded := submission.decoded(value, "extundo.")
	if decoded == nil {
		return nil
	}
	keys := []string{}
	for key := range decoded {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	shape := strings.Join(keys, ",")
	if shape != "accountId,messageId,submissionId" && shape != "accountId,archiveActionId,messageId,submissionId" {
		return nil
	}
	accountId, _ := decoded["accountId"].(string)
	submissionId, _ := decoded["submissionId"].(string)
	messageId := Threading{}.NormalizeMessageId(decoded["messageId"])
	actionId, _ := decoded["archiveActionId"].(string)
	if !submission.accountPattern().MatchString(accountId) || !submission.submissionPattern().MatchString(submissionId) || messageId == "" || (actionId != "" && !submission.actionPattern().MatchString(actionId)) {
		return nil
	}
	parsed := map[string]any{"accountId": accountId, "submissionId": submissionId, "messageId": messageId}
	if actionId != "" {
		parsed["archiveActionId"] = actionId
	}
	return parsed
}

// ProviderMessage reports whether value is a valid "extmsg." provider
// message id.
func (submission ExternalSubmission) ProviderMessage(value string) bool {
	decoded := submission.decoded(value, "extmsg.")
	if decoded == nil || len(decoded) != 2 {
		return false
	}
	accountId, _ := decoded["accountId"].(string)
	stableId, isText := decoded["stableId"].(string)
	return submission.accountPattern().MatchString(accountId) && isText && stableId != "" && len(stableId) <= 512 && !strings.ContainsAny(stableId, "\x00\r\n")
}

func (ExternalSubmission) decoded(value string, prefix string) map[string]any {
	if !strings.HasPrefix(value, prefix) || len(value) > 2048 {
		return nil
	}
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(value[len(prefix):], "="))
	decoded := map[string]any{}
	if err != nil || json.Unmarshal(data, &decoded) != nil {
		return nil
	}
	return decoded
}

// DeliveryRequest answers the provider-owned MIME input of a tracked
// request for one recipient (the request itself when untracked).
func (ExternalSubmission) DeliveryRequest(request map[string]any, trackingId string, recipient string) (map[string]any, error) {
	if trackingId == "" {
		return request, nil
	}
	tracked := TrackedHtml{TrackingId: trackingId, Recipient: recipient, OpenTracking: request["openTracking"] == true, LinkTracking: request["linkTracking"] == true}
	html, err := tracked.Prepared(RecordValues{}.Text(request["html"]))
	if err != nil {
		return nil, err
	}
	return DeliveryFields{}.Merged(request, map[string]any{"html": html}), nil
}

func (ExternalSubmission) accountPattern() *regexp.Regexp {
	return regexp.MustCompile(`^ext_[A-Za-z0-9_-]{20,80}$`)
}

func (ExternalSubmission) submissionPattern() *regexp.Regexp {
	return regexp.MustCompile(`^sub_[A-Za-z0-9_-]{20,80}$`)
}

func (ExternalSubmission) actionPattern() *regexp.Regexp {
	return regexp.MustCompile(`^[A-Za-z0-9_-]{1,512}$`)
}
