package outbox

import "encoding/json"

type documentCap struct{}

func (documentCap) bytes() int {
	return 512 * 1024
}

func (limit documentCap) checked(before Fields, staged Fields) (Fields, error) {
	encoded, err := json.Marshal(staged)
	if err != nil {
		return before, err
	}
	return limit.within(before, staged, len(encoded))
}

func (limit documentCap) within(before Fields, staged Fields, size int) (Fields, error) {
	if size > limit.bytes() {
		return before, OutboxFull{Bytes: size, Limit: limit.bytes()}
	}
	return staged, nil
}
