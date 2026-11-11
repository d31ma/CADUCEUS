package mailauth

import "strings"

type dkimHeaderPicker struct {
	groups map[string][]messageField
	used   map[string]int
}

func newDkimHeaderPicker(fields []messageField) dkimHeaderPicker {
	picker := dkimHeaderPicker{groups: map[string][]messageField{}, used: map[string]int{}}
	picker.group(fields)
	return picker
}

func (picker dkimHeaderPicker) group(fields []messageField) {
	for _, field := range fields {
		name := strings.ToLower(field.name)
		picker.groups[name] = append(picker.groups[name], field)
	}
}

func (picker dkimHeaderPicker) pick(names []string) []messageField {
	var picked []messageField
	for _, name := range names {
		picked = append(picked, picker.next(name)...)
	}
	return picked
}

func (picker dkimHeaderPicker) next(name string) []messageField {
	candidates := picker.groups[name]
	position := len(candidates) - 1 - picker.used[name]
	picker.used[name]++
	return picker.at(candidates, position)
}

func (dkimHeaderPicker) at(candidates []messageField, position int) []messageField {
	if position < 0 {
		return nil
	}
	return candidates[position : position+1]
}
