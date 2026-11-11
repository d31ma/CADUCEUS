package mailin

type BodyFormats struct{}

func (BodyFormats) Email() string {
	return "email"
}

func (BodyFormats) Text() string {
	return "text"
}
