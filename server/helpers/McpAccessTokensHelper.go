import "strings"

// McpAccessTokensHelper keeps the MCP access tokens of each mailbox, stored by
// the hash of the token.
// @Helper
type McpAccessTokensHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Put stores one token record and answers its document id.
func (helper *McpAccessTokensHelper) Put(record map[string]any) (string, error) {
	return helper.store().Put(record)
}

// ListForOwner answers the token records of one owner as {docId, ...record}.
func (helper *McpAccessTokensHelper) ListForOwner(ownerEmail string) ([]map[string]any, error) {
	return StoredRecords{}.Where(helper.store(), "ownerEmail", strings.ToLower(ownerEmail))
}

// FindByHash answers the token records with the token hash.
func (helper *McpAccessTokensHelper) FindByHash(tokenHash string) ([]map[string]any, error) {
	return StoredRecords{}.Where(helper.store(), "tokenHash", tokenHash)
}

// Update merges a patch into one token record.
func (helper *McpAccessTokensHelper) Update(documentId string, patch map[string]any) error {
	return helper.store().Patch(documentId, patch)
}

// Delete deletes one token record.
func (helper *McpAccessTokensHelper) Delete(documentId string) error {
	return helper.store().Delete(documentId)
}

func (helper *McpAccessTokensHelper) store() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().McpAccessTokens}
}
