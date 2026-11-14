package main

type Owners []Owner

func (owners Owners) Holding(number int) (Owner, bool) {
	for _, owner := range owners {
		if owner.Holds(number) {
			return owner, true
		}
	}
	return Owner{}, false
}

func (owners Owners) Kit() Owner {
	kit, _ := owners.Holding(0)
	return kit
}

func (owners Owners) Converted(unconverted map[string]bool) Owners {
	converted := Owners{}
	for _, owner := range owners {
		if !unconverted[owner.Name] {
			converted = append(converted, owner)
		}
	}
	return converted
}
