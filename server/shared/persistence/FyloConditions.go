// FyloConditions runs FYLO queries the shared collection methods do not
// cover: several conditions, which FYLO joins with OR, or operators other
// than $eq, such as {"expiresAt": {"$lte": now}}.
type FyloConditions struct{}

// Condition answers one FYLO condition.
func (FyloConditions) Condition(field string, operator string, value any) map[string]any {
	return map[string]any{field: map[string]any{operator: value}}
}

// Matching answers the documents of the collection that match any of the
// conditions, in FYLO's order.
func (FyloConditions) Matching(collection FyloCollection, conditions ...map[string]any) ([]FyloRecord, error) {
	operations := []any{}
	for _, condition := range conditions {
		operations = append(operations, condition)
	}
	documents, err := collection.Database.FindDocs(collection.Name, map[string]any{"$ops": operations})
	records := []FyloRecord{}
	for _, document := range documents {
		records = append(records, FyloRecord{}.Decoded(document))
	}
	return records, err
}
