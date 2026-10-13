import (
	"encoding/base64"
	"regexp"
	"strings"
	"unicode/utf16"
)

// ProviderMessageId makes the opaque, route-safe id of a provider message:
// "extmsg." and the base64url of {"accountId":...,"stableId":...} written
// as JSON.stringify writes it. Every decoded id is reauthorized by the owner.
// (The mailbox routes read these ids; this is the part the provider
// notification tick needs.)
type ProviderMessageId struct{}

// Of answers the id, or "" when the account id or stable id is not valid.
func (id ProviderMessageId) Of(accountId any, stableId any) string {
	account, isText := accountId.(string)
	stable, isStableText := stableId.(string)
	if !isText || !isStableText || !id.accountPattern().MatchString(account) || !id.validStableId(stable) {
		return ""
	}
	document := `{"accountId":` + id.quoted(account) + `,"stableId":` + id.quoted(stable) + `}`
	return "extmsg." + base64.RawURLEncoding.EncodeToString([]byte(document))
}

func (ProviderMessageId) accountPattern() *regexp.Regexp {
	return regexp.MustCompile(`^ext_[A-Za-z0-9_-]{20,80}$`)
}

func (ProviderMessageId) validStableId(value string) bool {
	units := len(utf16.Encode([]rune(value)))
	return units >= 1 && units <= 512 && !strings.ContainsAny(value, "\x00\r\n")
}

// quoted writes text as JSON.stringify does.
func (ProviderMessageId) quoted(text string) string {
	return StableJson{}.Quote(text)
}
