type BadRequest struct {
	ErrorCase
}

func (BadRequest) PasswordPolicy(rule string) *BadRequest {
	return &BadRequest{ErrorCase{508, "password_policy", "newPassword", rule}}
}

func (BadRequest) InvalidCode() *BadRequest {
	return &BadRequest{ErrorCase{509, "invalid_code", "code", "The 2-step code is not right."}}
}

func (BadRequest) SetupExpired() *BadRequest {
	return &BadRequest{ErrorCase{512, "setup_expired", "", "The passkey setup has expired. Start again."}}
}

func (BadRequest) InvalidPasskey() *BadRequest {
	return &BadRequest{ErrorCase{513, "invalid_passkey", "", "The passkey could not be added. Start again."}}
}

func (BadRequest) InvalidLabel() *BadRequest {
	return &BadRequest{ErrorCase{514, "invalid_request", "label", "A passkey name is 1 to 64 characters."}}
}

func (BadRequest) InvalidDomain() *BadRequest {
	return &BadRequest{ErrorCase{517, "invalid_request", "domainId", "The domain id must be a domain name or *."}}
}
