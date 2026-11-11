package textindex

type BadCursor struct{}

func (BadCursor) Error() string {
	return "the cursor is not valid"
}
