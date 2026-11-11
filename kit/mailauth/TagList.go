package mailauth

import (
	"slices"
	"strings"
)

type tagList struct {
	values map[string]string
}

func newTagList(text string) (tagList, error) {
	list := tagList{values: map[string]string{}}
	parts := slices.DeleteFunc(strings.Split(text, ";"), list.blank)
	return list, list.addAll(parts)
}

func (list tagList) addAll(parts []string) error {
	var err error
	for index := 0; index < len(parts) && err == nil; index++ {
		err = list.add(parts[index])
	}
	return err
}

func (list tagList) add(part string) error {
	name, value, err := list.pair(part)
	if err != nil {
		return err
	}
	err = list.fresh(name)
	if err != nil {
		return err
	}
	list.values[name] = value
	return nil
}

func (tagList) pair(part string) (string, string, error) {
	name, value, found := strings.Cut(part, "=")
	if !found {
		return "", "", failure{result: Outcomes{}.PermError()}
	}
	return strings.TrimSpace(name), strings.TrimSpace(value), nil
}

func (list tagList) fresh(name string) error {
	if list.has(name) {
		return failure{result: Outcomes{}.PermError()}
	}
	return nil
}

func (tagList) blank(part string) bool {
	return strings.TrimSpace(part) == ""
}

func (list tagList) has(name string) bool {
	_, found := list.values[name]
	return found
}

func (list tagList) missing(name string) bool {
	return !list.has(name)
}

func (list tagList) compact(name string) string {
	return strings.Join(strings.Fields(list.values[name]), "")
}
