type Conflict struct {
	ErrorCase
}

func (Conflict) LabelExists() *Conflict {
	return &Conflict{ErrorCase{510, "label_exists", "", "This passkey name is already used."}}
}

func (Conflict) TooManyPasskeys() *Conflict {
	return &Conflict{ErrorCase{511, "too_many", "", "You have the most passkeys allowed. Remove one first."}}
}
