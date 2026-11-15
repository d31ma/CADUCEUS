package syncing

import "mesail.dev/packages/core"

type changeOrder struct{}

func (changeOrder) later(first core.ChangeCursor, second core.ChangeCursor) core.ChangeCursor {
	if first.After(second) {
		return first
	}
	return second
}
