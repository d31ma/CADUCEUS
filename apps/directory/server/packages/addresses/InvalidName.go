package addresses

type InvalidName struct {
	Kind        string
	Reason      string
	ErrorNumber int
}

func (InvalidName) NotAscii(kind string) InvalidName {
	return InvalidName{Kind: kind, Reason: "must hold only ASCII characters", ErrorNumber: 37}
}

func (InvalidName) AddressWithoutAt() InvalidName {
	return InvalidName{Kind: NameKinds{}.Address(), Reason: "must hold an @ between the local part and the domain", ErrorNumber: 38}
}

func (InvalidName) DomainTooLong() InvalidName {
	return InvalidName{Kind: NameKinds{}.Domain(), Reason: "must be at most 253 characters", ErrorNumber: 39}
}

func (InvalidName) DomainLabels() InvalidName {
	return InvalidName{Kind: NameKinds{}.Domain(), Reason: "must be labels of 1 to 63 characters from a-z, 0-9 and - joined by single dots, with no - at the start or end of a label", ErrorNumber: 40}
}

func (InvalidName) LocalPartLength(kind string) InvalidName {
	return InvalidName{Kind: kind, Reason: "must be 1 to 64 characters", ErrorNumber: 41}
}

func (InvalidName) LocalPartCharacters(kind string) InvalidName {
	return InvalidName{Kind: kind, Reason: "may only hold a-z, 0-9, ., _ and -", ErrorNumber: 42}
}

func (InvalidName) LocalPartEnds(kind string) InvalidName {
	return InvalidName{Kind: kind, Reason: "must start and end with a letter or digit", ErrorNumber: 43}
}

func (InvalidName) LocalPartDots(kind string) InvalidName {
	return InvalidName{Kind: kind, Reason: "must not hold two dots in a row", ErrorNumber: 44}
}

func (InvalidName) RoleName() InvalidName {
	return InvalidName{Kind: NameKinds{}.Username(), Reason: "must not be a role name", ErrorNumber: 45}
}

func (invalid InvalidName) Error() string {
	return invalid.Kind + " " + invalid.Reason
}

func (invalid InvalidName) Number() int {
	return invalid.ErrorNumber
}
