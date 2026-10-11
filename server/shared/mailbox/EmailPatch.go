import (
	"errors"
	"strings"
)

// EmailPatch validates PUT /inbox/{id}: the mutable read, starred and folder
// fields of one email.
type EmailPatch struct{}

// Parsed answers the patch of a request body, or why it is invalid. An
// array reads as an object without fields, as typeof did.
func (patch EmailPatch) Parsed(input any) (map[string]any, error) {
	body, isObject := input.(map[string]any)
	if _, isList := input.([]any); isList {
		body, isObject = map[string]any{}, true
	}
	if !isObject || body == nil {
		return nil, errors.New("Update payload required")
	}
	parsed := map[string]any{}
	if value, given := body["folder"]; given {
		folder, isText := value.(string)
		if !isText || (TextValue{}).Trim(folder) == "" {
			return nil, errors.New("folder must be a non-empty string")
		}
		folder = strings.ToLower(TextValue{}.Trim(folder))
		if folder != "inbox" && folder != "archive" && folder != "trash" && (TextValue{}).Length(folder) > 60 {
			return nil, errors.New("folder is too long")
		}
		parsed["folder"] = folder
	}
	for _, field := range []string{"read", "starred"} {
		if value, given := body[field]; given {
			flag, isFlag := value.(bool)
			if !isFlag {
				return nil, errors.New(field + " must be a boolean")
			}
			parsed[field] = flag
		}
	}
	if len(parsed) == 0 {
		return nil, errors.New("No supported fields to update")
	}
	return parsed, nil
}
