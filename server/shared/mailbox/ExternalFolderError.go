// ExternalFolderError says a connected account exposes no folder of the
// requested name.
type ExternalFolderError struct {
	Folder string
}

func (failure *ExternalFolderError) Error() string {
	return "The connected account does not expose a " + failure.Folder + " folder"
}
