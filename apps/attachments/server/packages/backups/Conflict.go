package backups

type Conflict struct {
	Field       string
	Reason      string
	ErrorNumber int
}

func (Conflict) Replaced() Conflict {
	return Conflict{Field: "expectedVersion", Reason: "Another device replaced the backup. Download it, merge, and try again.", ErrorNumber: 3519}
}

func (Conflict) NotStarted() Conflict {
	return Conflict{Field: "file", Reason: "No upload of this backup version is waiting, or its link has expired. Start the backup again.", ErrorNumber: 3520}
}

func (conflict Conflict) Error() string {
	return conflict.Field + ": " + conflict.Reason
}
