package backups

type OtherDomain struct{}

func (OtherDomain) Error() string {
	return "the event belongs to another domain"
}
