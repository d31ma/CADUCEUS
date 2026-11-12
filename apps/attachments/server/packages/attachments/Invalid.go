package attachments

type Invalid struct {
	Field       string
	Reason      string
	ErrorNumber int
}

func (Invalid) FileName() Invalid {
	return Invalid{Field: "fileName", Reason: "Give a file name of 1 to 255 bytes without slashes or control characters.", ErrorNumber: 3501}
}

func (Invalid) MediaType() Invalid {
	return Invalid{Field: "mediaType", Reason: "Give a media type such as image/png.", ErrorNumber: 3502}
}

func (Invalid) Size() Invalid {
	return Invalid{Field: "size", Reason: "An attachment must have at least one byte.", ErrorNumber: 3503}
}

func (Invalid) Sha256() Invalid {
	return Invalid{Field: "sha256", Reason: "Give the SHA-256 of the file as 64 lowercase hex characters.", ErrorNumber: 3504}
}

func (invalid Invalid) Error() string {
	return invalid.Field + ": " + invalid.Reason
}
