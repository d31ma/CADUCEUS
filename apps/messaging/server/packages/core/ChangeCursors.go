package core

import (
	"strconv"
	"strings"
	"time"
)

type ChangeCursors struct{}

func (cursors ChangeCursors) Parse(text string) (ChangeCursor, bool) {
	nanos, rowId, _ := strings.Cut(text, cursors.separator())
	value, err := strconv.ParseInt(nanos, 10, 64)
	return ChangeCursor{At: time.Unix(0, value).UTC(), RowId: rowId}, cursors.valid(err, rowId)
}

func (cursors ChangeCursors) Of(change Change) ChangeCursor {
	at, _ := time.Parse(time.RFC3339Nano, change.LoggedAt)
	return ChangeCursor{At: at.UTC(), RowId: change.Id}
}

func (ChangeCursors) Stamp(at time.Time) string {
	return at.UTC().Format(time.RFC3339Nano)
}

func (ChangeCursors) valid(err error, rowId string) bool {
	if err != nil {
		return false
	}
	return rowId != ""
}

func (ChangeCursors) separator() string {
	return "."
}
