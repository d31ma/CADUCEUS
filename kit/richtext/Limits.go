package richtext

type Limits struct{}

func (Limits) MaxBlocks() int {
	return 2000
}

func (Limits) MaxCharacters() int {
	return 200000
}

func (Limits) MaxDepth() int {
	return 32
}
