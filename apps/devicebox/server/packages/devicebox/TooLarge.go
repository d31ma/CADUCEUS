package devicebox

type TooLarge struct {
	Message     string
	ErrorNumber int
}

func (TooLarge) Ciphertext() *TooLarge {
	return &TooLarge{Message: "The ciphertext is larger than 256 KiB.", ErrorNumber: 6009}
}

func (failure *TooLarge) Error() string {
	return failure.Message
}

func (failure *TooLarge) Number() int {
	return failure.ErrorNumber
}
