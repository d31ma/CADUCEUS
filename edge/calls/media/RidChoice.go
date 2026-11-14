package media

import "slices"

type RidChoice struct{}

func (choice RidChoice) For(quality int, available []string) string {
	order := choice.preferred(quality)
	return choice.firstAvailable(append(order, available...), available)
}

func (choice RidChoice) preferred(quality int) []string {
	order := slices.Clone(choice.names()[:quality+1])
	slices.Reverse(order)
	return order
}

func (RidChoice) firstAvailable(order []string, available []string) string {
	index := slices.IndexFunc(order, func(rid string) bool { return slices.Contains(available, rid) })
	if index < 0 {
		return ""
	}
	return order[index]
}

func (RidChoice) names() []string {
	return []string{"q", "h", "f"}
}
