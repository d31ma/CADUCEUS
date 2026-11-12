type Conflict struct {
	Code string
}

func (failure *Conflict) Error() string {
	return "this change conflicts with the person's passkeys"
}
