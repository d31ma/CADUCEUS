import "encoding/json"

// ThreadSharesRecords converts thread share documents. FYLO stores a
// share's email ids as JSON text; routes see them as a list.
type ThreadSharesRecords struct{}

// Serialize answers a share document with its emailIds as emailIdsJson
// (absent when the share has no emailIds).
func (ThreadSharesRecords) Serialize(share map[string]any) map[string]any {
	text := TextValue{}
	stored := text.Copy(share)
	delete(stored, "emailIds")
	if emailIds, present := share["emailIds"]; present {
		stored["emailIdsJson"] = text.Json(emailIds)
	}
	return stored
}

// SerializePatch answers the stored fields of a share patch: emailIds, when
// given, become emailIdsJson.
func (ThreadSharesRecords) SerializePatch(patch map[string]any) map[string]any {
	text := TextValue{}
	stored := text.Copy(patch)
	if text.Truthy(patch["emailIds"]) {
		stored["emailIdsJson"] = text.Json(patch["emailIds"])
		delete(stored, "emailIds")
	}
	return stored
}

// Deserialize answers one stored share with its email ids as a list of the
// texts its emailIdsJson holds.
func (ThreadSharesRecords) Deserialize(value map[string]any) map[string]any {
	text := TextValue{}
	emailIds := []any{}
	var parsed any
	if json.Unmarshal([]byte(text.Of(text.Or(value["emailIdsJson"], "[]"))), &parsed) == nil {
		for _, item := range text.List(parsed) {
			if _, isText := item.(string); isText {
				emailIds = append(emailIds, item)
			}
		}
	}
	result := text.Copy(value)
	result["emailIds"] = emailIds
	delete(result, "emailIdsJson")
	return result
}

// Listed answers the stored shares deserialized, each with its docId.
func (records ThreadSharesRecords) Listed(stored []FyloRecord) []map[string]any {
	shares := []map[string]any{}
	for _, record := range stored {
		shares = append(shares, FyloRecord{Id: record.Id, Data: records.Deserialize(record.Data)}.WithDocId())
	}
	return shares
}
