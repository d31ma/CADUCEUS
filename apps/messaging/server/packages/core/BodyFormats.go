package core

type BodyFormats struct{}

func (BodyFormats) Markdown() string {
	return "markdown"
}

func (BodyFormats) Text() string {
	return "text"
}

func (BodyFormats) Email() string {
	return "email"
}
