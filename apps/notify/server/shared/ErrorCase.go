type ErrorCase struct {
	number int
	code   string
	field  string
	text   string
}

func (failure ErrorCase) Error() string {
	return failure.text
}

func (failure ErrorCase) Number() int {
	return failure.number
}

func (failure ErrorCase) Code() string {
	return failure.code
}

func (failure ErrorCase) Field() string {
	return failure.field
}
