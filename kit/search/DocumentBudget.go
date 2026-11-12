package search

import "encoding/json"

type DocumentBudget struct{}

func (DocumentBudget) MaxBytes() int {
	return 480 << 10
}

func (DocumentBudget) TextCharacters() int {
	return 200000
}

func (DocumentBudget) LabelCharacters() int {
	return 1000
}

func (DocumentBudget) FileNameCharacters() int {
	return 255
}

func (DocumentBudget) MaxFileNames() int {
	return 100
}

func (DocumentBudget) Size(document Document) int {
	encoded, _ := json.Marshal(document)
	return len(encoded)
}

func (budget DocumentBudget) Over(document Document) bool {
	return budget.Size(document) > budget.MaxBytes()
}
