import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"regexp"
)

// InboxSourceRoute answers GET /inbox/{id}/source: the exact RFC 822 source
// of a message as an authenticated base64 payload, so the browser can show
// it without running its markup or save it as an .eml file.
type InboxSourceRoute struct {
	Parts MailboxParts
}

// Read answers the source with its EML metadata.
func (route InboxSourceRoute) Read(emailId string, request *http.Request) *http.Response {
	caller, refusal := route.Parts.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	if emailId == "" {
		return Answer{}.NotFound("Message source not found")
	}
	if accountId, stableId, external := (ExternalMessage{}).ParseId(emailId); external {
		return route.readExternal(caller, accountId, stableId)
	}
	_, email, refusal := route.Parts.Owned(caller, emailId)
	if refusal != nil {
		return refusal
	}
	source, err := route.Parts.Source(emailId)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if source == nil {
		return Answer{}.NotFound("Message source not found")
	}
	content, err := route.Parts.SourceBytes(source)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(route.payload(email["subject"], content, source["sha256"]))
}

func (route InboxSourceRoute) payload(subject any, content []byte, digest any) map[string]any {
	return map[string]any{
		"filename": route.SafeFilename(TextValue{}.Of(TextValue{}.Or(subject, "message"))) + ".eml", "contentType": "message/rfc822",
		"size": len(content), "sha256": digest, "contentBase64": base64.StdEncoding.EncodeToString(content),
	}
}

func (route InboxSourceRoute) readExternal(caller RouteCaller, accountId string, stableId string) *http.Response {
	accounts := route.Parts.Provider().Accounts
	source, err := accounts.MessageSource(caller.Token, accountId, stableId)
	if err != nil {
		return accounts.Failure(err)
	}
	encoded, isText := source["rawRFC822Base64"].(string)
	if !isText || len(encoded) > (ExternalMessage{}).longestSource() {
		return Answer{}.NotFound("Message source not found")
	}
	content := Base64Text{}.Decoded(encoded)
	if len(content) < 1 || len(content) > (ExternalMessage{}).largestMessage() {
		return Answer{}.NotFound("Message source not found")
	}
	message, _ := source["message"].(map[string]any)
	digest := sha256.Sum256(content)
	return Answer{}.Ok(route.payload(message["subject"], content, hex.EncodeToString(digest[:])))
}

// SafeFilename answers a subject as a portable file name (at most 120
// characters, "message" when nothing is left).
func (InboxSourceRoute) SafeFilename(value string) string {
	filename := regexp.MustCompile(`[/\\?%*:|"<>\x00-\x1f\x7f]`).ReplaceAllString(value, "-")
	filename = TextValue{}.Trim(TextValue{}.Collapsed(filename))
	filename = TextValue{}.Prefix(filename, 120)
	if filename == "" {
		return "message"
	}
	return filename
}
