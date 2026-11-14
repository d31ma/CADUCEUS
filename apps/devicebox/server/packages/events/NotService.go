package events

type NotService struct{}

func (NotService) Error() string {
	return "only a caller with the Service role may deliver events"
}
