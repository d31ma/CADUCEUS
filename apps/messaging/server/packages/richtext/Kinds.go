package richtext

type Kinds struct{}

func (Kinds) Paragraph() string {
	return "paragraph"
}

func (Kinds) Heading() string {
	return "heading"
}

func (Kinds) Quote() string {
	return "quote"
}

func (Kinds) ListItem() string {
	return "listItem"
}

func (Kinds) Code() string {
	return "code"
}

func (Kinds) Divider() string {
	return "divider"
}

func (Kinds) Image() string {
	return "image"
}

func (Kinds) Row() string {
	return "row"
}
