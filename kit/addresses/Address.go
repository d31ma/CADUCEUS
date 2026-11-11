package addresses

import "strings"

type Address struct{}

func (address Address) Parsed(input string) (AddressParts, error) {
	value, err := nameText{}.lowered(NameKinds{}.Address(), input)
	if err != nil {
		return AddressParts{}, err
	}
	localPart, domainId, found := nameText{}.splitAtLast(value)
	err = address.checkFound(found)
	if err != nil {
		return AddressParts{}, err
	}
	err = LocalPart{}.check(NameKinds{}.LocalPart(), localPart)
	if err != nil {
		return AddressParts{}, err
	}
	err = DomainName{}.check(domainId)
	if err != nil {
		return AddressParts{}, err
	}
	return AddressParts{LocalPart: localPart, DomainId: domainId}, nil
}

func (address Address) Normalized(input string) (string, error) {
	parts, err := address.Parsed(input)
	if err != nil {
		return "", err
	}
	return address.Primary(parts.LocalPart, parts.DomainId), nil
}

func (Address) Primary(username string, domainId string) string {
	return username + nameText{}.at() + domainId
}

func (Address) SameExternal(first string, second string) bool {
	return strings.EqualFold(strings.TrimSpace(first), strings.TrimSpace(second))
}

func (Address) checkFound(found bool) error {
	if !found {
		return InvalidName{Kind: NameKinds{}.Address(), Reason: "must hold an @ between the local part and the domain"}
	}
	return nil
}
