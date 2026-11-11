package core

type Visibilities struct{}

func (Visibilities) Public() string {
	return "public"
}

func (Visibilities) Private() string {
	return "private"
}
