package addresses

type Username struct{}

func (name Username) Normalized(input string) (string, error) {
	value, err := nameText{}.lowered(NameKinds{}.Username(), input)
	if err != nil {
		return "", err
	}
	err = name.check(value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (name Username) check(value string) error {
	err := LocalPart{}.check(NameKinds{}.Username(), value)
	if err != nil {
		return err
	}
	return name.checkRole(value)
}

func (Username) checkRole(value string) error {
	if (RoleNames{}).Contains(value) {
		return InvalidName{Kind: NameKinds{}.Username(), Reason: "must not be a role name"}
	}
	return nil
}
