package reading

import "mesail.dev/packages/core"

type refusals struct{}

func (refusals) noMessage() error {
	return &core.NotFound{Message: "no such message"}
}
