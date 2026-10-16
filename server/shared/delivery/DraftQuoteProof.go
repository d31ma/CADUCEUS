import (
	"regexp"
	"slices"
)

// DraftQuoteProof accepts compact composer provenance as a declared draft
// field. It grants no HTML authority by itself: the browser compares SHA-256
// of the sanitized canonical quote against this record before restoring
// quote controls. The draft API already requires the owning mailbox.
type DraftQuoteProof struct{}

// Parsed copies a bounded proof or answers why the complete snapshot is invalid.
// Missing and null values clear prior provenance on a draft merge update.
func (DraftQuoteProof) Parsed(value any) (map[string]any, string) {
	if value == nil {
		return nil, ""
	}
	proof, isObject := value.(map[string]any)
	problem := "quoteProof must contain only a reply/forward mode and a lowercase SHA-256 digest"
	if !isObject || len(proof) != 2 {
		return nil, problem
	}
	modeName, digestName := "mode", "sha256"
	mode, isMode := proof[modeName].(string)
	digest, isDigest := proof[digestName].(string)
	if !isMode || !isDigest || !slices.Contains([]string{"reply", "forward"}, mode) || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(digest) {
		return nil, problem
	}
	return map[string]any{modeName: mode, digestName: digest}, ""
}
