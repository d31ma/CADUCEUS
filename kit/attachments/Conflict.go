package attachments

type Conflict struct {
	Field       string
	Reason      string
	ErrorNumber int
}

func (Conflict) NotUploaded() Conflict {
	return Conflict{Field: "file", Reason: "The file has not been uploaded yet.", ErrorNumber: 3508}
}

func (Conflict) SizeMismatch() Conflict {
	return Conflict{Field: "size", Reason: "The uploaded file is smaller than the size you gave.", ErrorNumber: 3509}
}

func (Conflict) Sha256Mismatch() Conflict {
	return Conflict{Field: "sha256", Reason: "The uploaded file does not match the SHA-256 you gave.", ErrorNumber: 3510}
}

func (Conflict) NotReady() Conflict {
	return Conflict{Field: "state", Reason: "The attachment is not ready to download.", ErrorNumber: 3511}
}

func (conflict Conflict) Error() string {
	return conflict.Field + ": " + conflict.Reason
}
