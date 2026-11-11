package mailin

type Missing struct {
	Message string
}

func (missing Missing) Error() string {
	return missing.Message
}
