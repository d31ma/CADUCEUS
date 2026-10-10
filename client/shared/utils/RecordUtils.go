import "strconv"

// RecordUtils reads loosely typed JSON records from the mail API without
// failing the whole answer when one field has an unexpected shape.
type RecordUtils struct{}

func (RecordUtils) Text(record map[string]any, key string) string {
	return RecordUtils{}.text(record[key])
}

func (RecordUtils) Flag(record map[string]any, key string) bool {
	flag, _ := record[key].(bool)
	return flag
}

func (RecordUtils) Number(record map[string]any, key string) float64 {
	number, _ := record[key].(float64)
	return number
}

func (RecordUtils) HasNumber(record map[string]any, key string) bool {
	_, found := record[key].(float64)
	return found
}

func (RecordUtils) IsList(record map[string]any, key string) bool {
	_, found := record[key].([]any)
	return found
}

func (RecordUtils) Texts(record map[string]any, key string) []string {
	return RecordUtils{}.texts(record[key])
}

func (RecordUtils) Record(record map[string]any, key string) map[string]any {
	found, _ := record[key].(map[string]any)
	return found
}

func (RecordUtils) Records(record map[string]any, key string) []map[string]any {
	items, _ := record[key].([]any)
	return RecordUtils{}.records(items)
}

func (RecordUtils) Object(value BehaviorValue) map[string]any {
	found, _ := value.(map[string]any)
	return RecordUtils{}.orEmpty(found)
}

func (RecordUtils) Merge(record map[string]any, patch map[string]any) map[string]any {
	merged := map[string]any{}
	RecordUtils{}.copyInto(merged, record)
	RecordUtils{}.copyInto(merged, patch)
	return merged
}

func (RecordUtils) text(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	}
	return ""
}

func (RecordUtils) texts(value any) []string {
	switch typed := value.(type) {
	case string:
		return RecordUtils{}.single(typed)
	case []any:
		return RecordUtils{}.textItems(typed)
	}
	return []string{}
}

func (RecordUtils) single(text string) []string {
	found := []string{}
	if text != "" {
		found = append(found, text)
	}
	return found
}

func (RecordUtils) textItems(items []any) []string {
	found := []string{}
	for _, item := range items {
		found = append(found, RecordUtils{}.itemText(item)...)
	}
	return found
}

func (RecordUtils) itemText(item any) []string {
	text, found := item.(string)
	if found {
		return []string{text}
	}
	return []string{}
}

func (RecordUtils) records(items []any) []map[string]any {
	found := []map[string]any{}
	for _, item := range items {
		found = append(found, RecordUtils{}.itemRecord(item)...)
	}
	return found
}

func (RecordUtils) itemRecord(item any) []map[string]any {
	record, found := item.(map[string]any)
	if found {
		return []map[string]any{record}
	}
	return []map[string]any{}
}

func (RecordUtils) orEmpty(record map[string]any) map[string]any {
	if record == nil {
		return map[string]any{}
	}
	return record
}

func (RecordUtils) copyInto(target map[string]any, source map[string]any) {
	for key, value := range source {
		target[key] = value
	}
}
