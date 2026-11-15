package backups

type Missing struct{}

func (Missing) Error() string {
	return "the backup was not found"
}
