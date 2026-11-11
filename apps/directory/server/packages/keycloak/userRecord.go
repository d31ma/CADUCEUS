package keycloak

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type userRecord map[string]json.RawMessage

func (record userRecord) setEnabled(enabled bool) {
	record[record.enabledKey()] = json.RawMessage(strconv.FormatBool(enabled))
}

func (record userRecord) requirePasswordChange() error {
	attributes := map[string]json.RawMessage{}
	err := json.Unmarshal(record.attributesText(), &attributes)
	if err != nil {
		return fmt.Errorf("the Keycloak user attributes are not a JSON object: %w", err)
	}
	attributes[mustChangePassword{}.Key()] = mustChangePassword{}.Raw()
	text, marshalError := json.Marshal(attributes)
	record[record.attributesKey()] = text
	return marshalError
}

func (record userRecord) attribute(key string) (string, error) {
	attributes := map[string][]string{}
	err := json.Unmarshal(record.attributesText(), &attributes)
	if err != nil {
		return "", fmt.Errorf("the Keycloak user attributes are not a JSON object of lists: %w", err)
	}
	return record.first(attributes[key]), nil
}

func (userRecord) first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (record userRecord) attributesText() json.RawMessage {
	text := record[record.attributesKey()]
	if len(text) == 0 || string(text) == "null" {
		return json.RawMessage("{}")
	}
	return text
}

func (userRecord) enabledKey() string {
	return "enabled"
}

func (userRecord) attributesKey() string {
	return "attributes"
}
