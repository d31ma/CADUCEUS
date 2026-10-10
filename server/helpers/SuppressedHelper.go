// SuppressedHelper keeps the addresses outbound mail must not reach.
// @Helper
type SuppressedHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// List answers every suppressed address record, in FYLO's order.
func (helper *SuppressedHelper) List() ([]map[string]any, error) {
	documents, err := helper.store().All()
	return StoredRecords{}.Listed(documents), err
}

// AsSet answers the suppressed addresses.
func (helper *SuppressedHelper) AsSet() (map[string]bool, error) {
	records, err := helper.List()
	addresses := map[string]bool{}
	for _, record := range records {
		addresses[RecordValues{}.Text(record[helper.addressField()])] = true
	}
	return addresses, err
}

// Suppress stores the address with a reason unless it is already suppressed.
func (helper *SuppressedHelper) Suppress(address string, reason string) error {
	existing, err := helper.store().Where(helper.addressField(), address)
	if err != nil || len(existing) > 0 {
		return err
	}
	_, err = helper.store().Put(map[string]any{helper.addressField(): address, "reason": reason, "suppressedAt": EpochTime{}.NowText()})
	return err
}

// Delete deletes every suppression of the address.
func (helper *SuppressedHelper) Delete(address string) error {
	existing, err := helper.store().Where(helper.addressField(), address)
	if err != nil {
		return err
	}
	for _, document := range existing {
		if err := helper.store().Delete(document.Id); err != nil {
			return err
		}
	}
	return nil
}

func (SuppressedHelper) addressField() string { return "address" }

func (helper *SuppressedHelper) store() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().Suppressed}
}
