// Open the file picker for EML files to import into this mailbox and folder.
type ImportPickRequested struct {
	Mailbox       string `json:"mailbox"`
	Folder        string `json:"folder"`
	Authorization string `json:"authorization"`
}
