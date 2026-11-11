import "time"

// @Service
type SearchTimeService struct{}

func (service *SearchTimeService) LabelOf(createdAt string) string {
	instant, failure := time.Parse(time.RFC3339Nano, createdAt)
	return service.labelUnlessFailed(instant, failure, createdAt)
}

func (service *SearchTimeService) labelUnlessFailed(instant time.Time, failure error, createdAt string) string {
	if failure != nil {
		return createdAt
	}
	return instant.UTC().Format(service.layout())
}

func (service *SearchTimeService) layout() string {
	return "2 Jan 2006, 15:04 UTC"
}
