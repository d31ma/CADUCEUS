package recipients

type Kinds struct{}

func (Kinds) Member() Kind {
	return "member"
}

func (Kinds) Channel() Kind {
	return "channel"
}

func (Kinds) OtherDomain() Kind {
	return "otherDomain"
}

func (Kinds) External() Kind {
	return "external"
}
