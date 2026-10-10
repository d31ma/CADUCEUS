import "strings"

// ThreadSharesHelper keeps the conversation shares a mailbox has made.
// @Helper
type ThreadSharesHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Put stores a share and answers its document id.
func (helper *ThreadSharesHelper) Put(share map[string]any) (string, error) {
	return helper.shares().Put(ThreadSharesRecords{}.Serialize(share))
}

// ListForOwner answers the owner's shares, each with its docId.
func (helper *ThreadSharesHelper) ListForOwner(ownerEmail string) ([]map[string]any, error) {
	return helper.listed(helper.shares().Where("ownerEmail", strings.ToLower(ownerEmail)))
}

// FindById answers the document id and share with a logical id, or "" and nil.
func (helper *ThreadSharesHelper) FindById(id string) (string, map[string]any, error) {
	record, err := helper.shares().First("id", id)
	if err != nil || record.Id == "" {
		return "", nil, err
	}
	return record.Id, ThreadSharesRecords{}.Deserialize(record.Data), nil
}

// FindByTokenHash answers the shares whose token has the hash, each with
// its docId.
func (helper *ThreadSharesHelper) FindByTokenHash(tokenHash string) ([]map[string]any, error) {
	return helper.listed(helper.shares().Where("tokenHash", tokenHash))
}

// Update merges fields into one share.
func (helper *ThreadSharesHelper) Update(documentId string, patch map[string]any) error {
	return helper.shares().Patch(documentId, ThreadSharesRecords{}.SerializePatch(patch))
}

func (*ThreadSharesHelper) listed(stored []FyloRecord, failure error) ([]map[string]any, error) {
	if failure != nil {
		return nil, failure
	}
	return ThreadSharesRecords{}.Listed(stored), nil
}

func (helper *ThreadSharesHelper) shares() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().ThreadShares}
}
