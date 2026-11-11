type NotService struct{}

func (failure *NotService) Error() string {
	return "only a caller with the Service role may call this route"
}
