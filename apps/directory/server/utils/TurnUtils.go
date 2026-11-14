import (
	"cmp"
	"slices"
	"strings"
)

type TurnUtils struct{}

func (TurnUtils) Visit(ids []string, listFailure error, offset int, step func(index int) error) error {
	order := TurnUtils{}.from(ids, offset)
	failure := BatchUtils{}.Each(len(order), func(position int) error { return step(order[position]) })
	return cmp.Or(listFailure, failure)
}

func (TurnUtils) from(ids []string, offset int) []int {
	order := TurnUtils{}.sorted(ids)
	start := offset % max(len(order), 1)
	return slices.Concat(order[start:], order[:start])
}

func (TurnUtils) sorted(ids []string) []int {
	order := make([]int, len(ids))
	for index := range order {
		order[index] = index
	}
	slices.SortStableFunc(order, func(left int, right int) int { return strings.Compare(ids[left], ids[right]) })
	return order
}
