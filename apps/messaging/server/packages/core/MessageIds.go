package core

import (
	wireIds "mesail.dev/packages/ids"
	"time"
)

type MessageIds struct{}

func (ids MessageIds) Wire(month string, storedId string) string {
	return ids.wire().Add(ids.prefix(), month+storedId)
}

func (ids MessageIds) Parse(wireId string) (string, string, error) {
	stored, err := ids.wire().Strip(ids.prefix(), wireId)
	if err != nil {
		return "", "", &NotFound{Message: "no such message"}
	}
	return ids.split(stored)
}

func (ids MessageIds) Month(at time.Time) string {
	return at.UTC().Format(ids.monthLayout())
}

func (ids MessageIds) split(stored string) (string, string, error) {
	if len(stored) <= len(ids.monthLayout()) {
		return "", "", &NotFound{Message: "no such message"}
	}
	return stored[:len(ids.monthLayout())], stored[len(ids.monthLayout()):], nil
}

func (MessageIds) wire() wireIds.WireId {
	return wireIds.WireId{}
}

func (MessageIds) prefix() string {
	return "msg"
}

func (MessageIds) monthLayout() string {
	return "200601"
}
