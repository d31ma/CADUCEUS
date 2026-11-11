package ids

import "strings"

type WireId struct{}

func (WireId) Add(prefix string, storedId string) string {
	return prefix + "_" + storedId
}

func (wire WireId) Strip(prefix string, wireId string) (string, error) {
	storedId, found := strings.CutPrefix(wireId, prefix+"_")
	return wire.checked(storedId, found, prefix)
}

func (WireId) checked(storedId string, found bool, prefix string) (string, error) {
	if !found || storedId == "" {
		return "", WrongIdPrefix{Expected: prefix}
	}
	return storedId, nil
}
