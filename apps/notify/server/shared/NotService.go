type NotService struct {
	ErrorCase
}

func (NotService) ServiceOnly() *NotService {
	return &NotService{ErrorCase{4503, "forbidden", "", "Only another Mesail service may call this route."}}
}

func (NotService) Kit(text string, number int) *NotService {
	return &NotService{ErrorCase{number, "forbidden", "", text}}
}
