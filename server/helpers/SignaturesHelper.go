// SignaturesHelper reads and writes message signatures.
// @Helper
type SignaturesHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

func (helper *SignaturesHelper) signatures() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().Signatures}
}

// Put stores a signature and answers its document id.
func (helper *SignaturesHelper) Put(entry map[string]any) (string, error) {
	return helper.signatures().Put(entry)
}

// ListForOwner answers the owner's signatures.
func (helper *SignaturesHelper) ListForOwner(owner string) ([]FyloRecord, error) {
	return helper.signatures().Where("owner", owner)
}

// ReleaseDefaults removes newly claimed addresses from the defaults of the
// owner's other signatures, since an address has at most one default.
func (helper *SignaturesHelper) ReleaseDefaults(owner string, addresses []string, excludeId string) error {
	if len(addresses) == 0 {
		return nil
	}
	records, err := helper.ListForOwner(owner)
	if err != nil {
		return err
	}
	return helper.signatures().PatchEach(SignatureDefaults{}.Released(records, addresses, excludeId))
}

// FindById answers the document id and signature with a logical id, or ""
// and nil.
func (helper *SignaturesHelper) FindById(id string) (string, map[string]any, error) {
	return helper.signatures().FindById(id)
}

// Update merges fields into one signature.
func (helper *SignaturesHelper) Update(documentId string, patch map[string]any) error {
	return helper.signatures().Patch(documentId, patch)
}

// Delete deletes one signature.
func (helper *SignaturesHelper) Delete(documentId string) error {
	return helper.signatures().Delete(documentId)
}
