import "fmt"

// MailActionLabels validates and applies label operations: nested label
// paths added and removed together, at most 100 of them.
type MailActionLabels struct{}

func (MailActionLabels) mostLabels() int { return 100 }

// Operation answers a validated {add, remove} operation, or why it is
// invalid.
func (labels MailActionLabels) Operation(value any) (map[string]any, error) {
	fields, isObject := value.(map[string]any)
	if !isObject {
		return nil, &MailActionError{Message: "labels must contain add or remove arrays", Status: 400}
	}
	for _, key := range (MailActionRules{}).sortedKeys(fields) {
		if key != "add" && key != "remove" {
			return nil, &MailActionError{Message: "Unsupported labels field: " + key, Status: 400}
		}
	}
	add, addErr := labels.requested(fields["add"])
	remove, removeErr := labels.requested(fields["remove"])
	if addErr != nil {
		return nil, addErr
	}
	if removeErr != nil {
		return nil, removeErr
	}
	if len(add)+len(remove) == 0 {
		return nil, &MailActionError{Message: "labels must add or remove at least one path", Status: 400}
	}
	if len(add)+len(remove) > labels.mostLabels() {
		return nil, &MailActionError{Message: fmt.Sprintf("labels cannot contain more than %d paths", labels.mostLabels()), Status: 400}
	}
	for _, path := range add {
		if (TextValue{}).Includes(remove, path) {
			return nil, &MailActionError{Message: "A label cannot be added and removed together", Status: 400}
		}
	}
	return map[string]any{"add": add, "remove": remove}, nil
}

func (MailActionLabels) requested(value any) ([]string, error) {
	if value == nil {
		return []string{}, nil
	}
	list, isList := value.([]any)
	if !isList {
		return nil, &MailActionError{Message: "labels add and remove must be arrays", Status: 400}
	}
	paths := []string{}
	for _, item := range list {
		path := CategoryPath{}.Normalized(item)
		if path == "" {
			return nil, &MailActionError{Message: "labels contain an invalid category path", Status: 400}
		}
		paths = append(paths, path)
	}
	if len(MailAddresses{}.Unique(paths)) != len(paths) {
		return nil, &MailActionError{Message: "labels cannot contain duplicates", Status: 400}
	}
	return paths, nil
}

// Applied answers current labels after an operation, keeping their order
// and appending new labels.
func (labels MailActionLabels) Applied(current any, operation any) []string {
	fields, _ := operation.(map[string]any)
	remove := labels.texts(fields["remove"])
	kept := []string{}
	for _, label := range labels.Stored(current) {
		if !(TextValue{}).Includes(remove, label) {
			kept = append(kept, label)
		}
	}
	for _, label := range labels.texts(fields["add"]) {
		if !(TextValue{}).Includes(kept, label) {
			kept = append(kept, label)
		}
	}
	return kept
}

func (MailActionLabels) texts(value any) []string {
	texts := []string{}
	switch list := value.(type) {
	case []string:
		return list
	case []any:
		for _, item := range list {
			texts = append(texts, TextValue{}.Of(item))
		}
	}
	return texts
}

// Stored answers labels read from FYLO or a journal: unique safe paths, at
// most 100.
func (labels MailActionLabels) Stored(value any) []string {
	list, isList := RecordValues{}.List(value)
	if !isList {
		return []string{}
	}
	paths := []string{}
	for _, item := range list {
		if path := (CategoryPath{}).Normalized(item); path != "" {
			paths = append(paths, path)
		}
	}
	unique := MailAddresses{}.Unique(paths)
	if len(unique) > labels.mostLabels() {
		unique = unique[:labels.mostLabels()]
	}
	return unique
}
