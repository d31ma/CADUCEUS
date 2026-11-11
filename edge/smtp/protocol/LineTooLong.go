package protocol

type LineTooLong struct{}

func (LineTooLong) Error() string {
	return "the command line is too long"
}
