package syncing

type changePage struct {
	messageIds []string
	next       string
	more       bool
	fresh      bool
}
