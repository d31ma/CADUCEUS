package attachments

type Forbidden struct{}

func (Forbidden) Error() string {
	return "only a service may read attachment details"
}
