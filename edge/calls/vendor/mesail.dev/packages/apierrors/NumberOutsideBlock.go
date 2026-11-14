package apierrors

import "fmt"

type NumberOutsideBlock struct {
	Number int
	Block  NumberBlock
}

func (outside NumberOutsideBlock) Error() string {
	return fmt.Sprintf("MS%04d is outside the block MS%04d-MS%04d", outside.Number, outside.Block.First, outside.Block.Last)
}
