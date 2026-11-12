type NotFound struct {
	ErrorCase
}

func (NotFound) Passkey() *NotFound {
	return &NotFound{ErrorCase{515, "not_found", "", "That passkey was not found."}}
}

func (NotFound) Domain() *NotFound {
	return &NotFound{ErrorCase{516, "not_found", "", "That domain was not found."}}
}
