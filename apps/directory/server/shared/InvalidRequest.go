type InvalidRequest struct {
	ErrorCase
}

func (request InvalidRequest) LimitOutOfRange() *InvalidRequest      { return request.invalid(1007) }
func (request InvalidRequest) LimitNotNumber() *InvalidRequest       { return request.invalid(1008) }
func (request InvalidRequest) BadCursor() *InvalidRequest            { return request.invalid(1009) }
func (request InvalidRequest) AddressOutsideDomain() *InvalidRequest { return request.invalid(1016) }
func (request InvalidRequest) NoChannelId() *InvalidRequest          { return request.invalid(1022) }
func (request InvalidRequest) TooManyAddresses() *InvalidRequest     { return request.invalid(1023) }
func (request InvalidRequest) NoNewPersonName() *InvalidRequest      { return request.invalid(1026) }
func (request InvalidRequest) NoFirstPassword() *InvalidRequest      { return request.invalid(1027) }
func (request InvalidRequest) BadState() *InvalidRequest             { return request.invalid(1035) }
func (request InvalidRequest) NoNewPassword() *InvalidRequest        { return request.invalid(1036) }
func (request InvalidRequest) NoPersonId() *InvalidRequest           { return request.invalid(1041) }
func (request InvalidRequest) NoDisplayName() *InvalidRequest        { return request.invalid(1042) }
func (request InvalidRequest) TooManyMemberships() *InvalidRequest   { return request.invalid(1043) }

func (InvalidRequest) BadDomainName(reason string) *InvalidRequest {
	return InvalidRequest{}.invalidWith(1011, "name", reason)
}

func (InvalidRequest) BadName(field string, reason string) *InvalidRequest {
	return InvalidRequest{}.invalidWith(1019, field, reason)
}

func (InvalidRequest) BadAddresses(reason string) *InvalidRequest {
	return InvalidRequest{}.invalidWith(1024, "addresses", reason)
}

func (InvalidRequest) invalid(number int) *InvalidRequest {
	fieldAndText := InvalidRequest{}.fieldsAndTexts()[number]
	return &InvalidRequest{ErrorCase{number, "invalid_request", fieldAndText[0], fieldAndText[1]}}
}

func (InvalidRequest) invalidWith(number int, field string, text string) *InvalidRequest {
	return &InvalidRequest{ErrorCase{number, "invalid_request", field, text}}
}

func (InvalidRequest) fieldsAndTexts() map[int][2]string {
	return map[int][2]string{
		1007: {"limit", "limit must be between 1 and 100"},
		1008: {"limit", "limit must be a whole number"},
		1009: {"cursor", "the cursor is not one this service gave out"},
		1016: {"address", "the address must be in your own domain"},
		1022: {"channelId", "a channel id is required"},
		1023: {"addresses", "at most 100 addresses can be resolved at once"},
		1026: {"displayName", "a display name is required for a new person"},
		1027: {"firstPassword", "a first password is required for a new person"},
		1035: {"state", "state must be active or disabled"},
		1036: {"newPassword", "a new password is required"},
		1041: {"personId", "a person id is required"},
		1042: {"displayName", "a display name is required"},
		1043: {"memberships", "a seed call adds at most 2 domains; seed the person again for more"},
	}
}
