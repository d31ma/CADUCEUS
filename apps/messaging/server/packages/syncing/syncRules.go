package syncing

import (
	"errors"

	"mesail.dev/packages/core"
)

type syncRules struct{}

func (rules syncRules) check(cursors []core.SyncCursor) error {
	if len(cursors) < 1 || len(cursors) > rules.most() {
		return core.InvalidRequest{}.SyncCursorCount()
	}
	return nil
}

func (syncRules) gone(failure error) bool {
	var missing *core.NotFound
	return errors.As(failure, &missing)
}

func (syncRules) most() int {
	return 50
}
