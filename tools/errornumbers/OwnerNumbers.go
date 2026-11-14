package main

type OwnerNumbers struct {
	Owner   Owner
	Numbers []UsedNumber
}

func (owned OwnerNumbers) Uses(number int) bool {
	for _, used := range owned.Numbers {
		if used.Number == number {
			return true
		}
	}
	return false
}
