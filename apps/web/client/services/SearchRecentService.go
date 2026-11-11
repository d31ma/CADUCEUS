import (
	"slices"
	"strings"
)

// @Service
type SearchRecentService struct{}

func (service *SearchRecentService) With(recent []string, query string) []string {
	return service.capped(service.withFirst(recent, strings.TrimSpace(query)))
}

func (service *SearchRecentService) withFirst(recent []string, query string) []string {
	if query == "" {
		return slices.Clone(recent)
	}
	return append([]string{query}, service.without(recent, query)...)
}

func (service *SearchRecentService) without(recent []string, query string) []string {
	return slices.DeleteFunc(slices.Clone(recent), func(earlier string) bool {
		return strings.EqualFold(earlier, query)
	})
}

func (service *SearchRecentService) capped(recent []string) []string {
	return recent[:min(len(recent), service.mostKept())]
}

func (service *SearchRecentService) mostKept() int {
	return 5
}
