package core

type Renderer interface {
	Markdown(body string) (RichText, error)
}
