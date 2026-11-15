package core

import (
	"cmp"
	"fmt"
	"time"
)

type ChangeCursor struct {
	At    time.Time
	RowId string
}

func (cursor ChangeCursor) Text() string {
	return fmt.Sprintf("%019d.%s", cursor.At.UnixNano(), cursor.RowId)
}

func (cursor ChangeCursor) Compare(other ChangeCursor) int {
	return cmp.Or(cursor.At.Compare(other.At), cmp.Compare(cursor.RowId, other.RowId))
}

func (cursor ChangeCursor) After(other ChangeCursor) bool {
	return cursor.Compare(other) > 0
}
