package addresses

type Login struct{}

func (login Login) Parsed(input string) (LoginParts, error) {
	value, err := nameText{}.lowered(NameKinds{}.Username(), input)
	if err != nil {
		return LoginParts{}, err
	}
	username, domainId, found := nameText{}.splitAtLast(value)
	err = Username{}.check(username)
	if err != nil {
		return LoginParts{}, err
	}
	err = login.checkDomain(domainId, found)
	if err != nil {
		return LoginParts{}, err
	}
	return LoginParts{Username: username, DomainId: domainId}, nil
}

func (Login) checkDomain(domainId string, found bool) error {
	if !found {
		return nil
	}
	return DomainName{}.check(domainId)
}
