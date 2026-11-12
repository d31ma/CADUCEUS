package apierrors

type NumberBlock struct {
	First int
	Last  int
}

func (block NumberBlock) Holds(number int) bool {
	return block.First <= number && number <= block.Last
}
