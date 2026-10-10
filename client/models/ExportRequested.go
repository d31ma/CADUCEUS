// Stream the folder into an mbox file or an archive of EML files.
type ExportRequested struct {
	Format         string        `json:"format"`
	Folder         string        `json:"folder"`
	FilenameDomain string        `json:"filenameDomain"`
	Scopes         []ExportScope `json:"scopes"`
	Authorization  string        `json:"authorization"`
}
