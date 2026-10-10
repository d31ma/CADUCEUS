// EmailListFilters narrows a mailbox listing. Nil flags and an empty text
// leave that filter off; a nil Limit lists every email after Offset.
type EmailListFilters struct {
	Query          string
	Folder         string
	FolderByDomain map[string]string
	Read           *bool
	Starred        *bool
	HasAttachment  *bool
	Limit          *int
	Offset         int
	Since          string
	OwnerEmail     string
}
