package core

type ParticipantKinds struct{}

func (ParticipantKinds) Member() string {
	return "member"
}

func (ParticipantKinds) OtherDomain() string {
	return "otherDomain"
}

func (ParticipantKinds) External() string {
	return "external"
}
