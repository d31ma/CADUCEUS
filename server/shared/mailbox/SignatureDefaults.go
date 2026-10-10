// SignatureDefaults keeps each address the default of at most one signature.
type SignatureDefaults struct{}

// Released answers the patches that remove claimed addresses from the
// defaults of every signature but the excluded one.
func (SignatureDefaults) Released(records []FyloRecord, addresses []string, excludeId string) []FyloRecord {
	text := TextValue{}
	patches := []FyloRecord{}
	for _, record := range records {
		if record.Data["id"] == excludeId {
			continue
		}
		current := text.List(record.Data["defaultFor"])
		next := []any{}
		for _, address := range current {
			if !text.Includes(addresses, address) {
				next = append(next, address)
			}
		}
		if len(next) != len(current) {
			patches = append(patches, FyloRecord{Id: record.Id, Data: map[string]any{"defaultFor": next, "updatedAt": RandomText{}.Millis()}})
		}
	}
	return patches
}
