import "mesail.dev/packages/recipients"

// @Service
type AddressRulesService struct {
}

func (rules *AddressRulesService) OwnAddress(domainId string, raw string) (string, error) {
	address, invalid := recipients.Addresses{}.Normalize(raw)
	return rules.owned(address, invalid, domainId)
}

func (rules *AddressRulesService) owned(address string, invalid error, domainId string) (string, error) {
	switch {
	case invalid != nil, (recipients.Addresses{}).Domain(address) != domainId:
		return "", &InvalidRequest{Field: "address", Message: "the address must be in your own domain"}
	}
	return address, nil
}
