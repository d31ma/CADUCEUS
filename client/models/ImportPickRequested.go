// The mailbox and folder that chosen EML files import into. Open asks the
// bridge to open the file picker now; without it the target only changes.
type ImportPickRequested struct {
	Mailbox       string `json:"mailbox"`
	Folder        string `json:"folder"`
	Authorization string `json:"authorization"`
	Open          bool   `json:"open"`
}
