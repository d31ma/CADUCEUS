package syncing

import "mesail.dev/packages/core"

type changeHeads struct {
	heads core.ChangeHeadStore
}

func (heads changeHeads) raised(domainId string, rows []core.Change) error {
	if len(rows) == 0 {
		return nil
	}
	head, found, err := heads.heads.Find(domainId, rows[0].ConversationId)
	if err != nil {
		return core.Unavailable{}.Store(err)
	}
	return heads.writtenIfMoved(head, heads.moved(head, domainId, rows), found)
}

func (heads changeHeads) writtenIfMoved(before core.ChangeHead, after core.ChangeHead, found bool) error {
	if found && before.LastChangeId == after.LastChangeId {
		return nil
	}
	return heads.written(after, found)
}

func (heads changeHeads) moved(head core.ChangeHead, domainId string, rows []core.Change) core.ChangeHead {
	cursors := core.ChangeCursors{}
	newest := heads.newest(rows)
	current, valid := cursors.Parse(head.LastChangeId)
	head.DomainId = domainId
	head.ConversationId = rows[0].ConversationId
	head.LastChangeId = heads.later(current, valid, newest).Text()
	return head
}

func (changeHeads) newest(rows []core.Change) core.ChangeCursor {
	cursors := core.ChangeCursors{}
	newest := cursors.Of(rows[0])
	for _, row := range rows[1:] {
		newest = changeOrder{}.later(newest, cursors.Of(row))
	}
	return newest
}

func (changeHeads) later(current core.ChangeCursor, valid bool, newest core.ChangeCursor) core.ChangeCursor {
	if valid && current.After(newest) {
		return current
	}
	return newest
}

func (heads changeHeads) written(head core.ChangeHead, found bool) error {
	if found {
		return heads.unavailable(heads.heads.Save(head))
	}
	_, err := heads.heads.Insert(head)
	return heads.unavailable(err)
}

func (changeHeads) unavailable(failure error) error {
	if failure == nil {
		return nil
	}
	return core.Unavailable{}.Store(failure)
}
