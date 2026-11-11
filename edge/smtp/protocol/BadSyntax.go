package protocol

type BadSyntax struct{}

func (BadSyntax) Error() string {
	return "syntax error in parameters"
}
