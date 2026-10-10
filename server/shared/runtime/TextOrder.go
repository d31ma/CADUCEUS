import (
	"strings"
	"unicode"
)

// TextOrder compares text close to how String.prototype.localeCompare did
// under the root collation: white space, then punctuation and symbols in
// collation order, then digits, then letters without regard to case, with
// lower case first when two texts differ only in case.
type TextOrder struct{}

// Compare answers a negative number, zero or a positive number as left sorts
// before, with or after right.
func (order TextOrder) Compare(left string, right string) int {
	if primary := order.compareRunes([]rune(strings.ToLower(left)), []rune(strings.ToLower(right))); primary != 0 {
		return primary
	}
	return strings.Compare(right, left)
}

func (order TextOrder) compareRunes(left []rune, right []rune) int {
	for index := 0; index < len(left) && index < len(right); index++ {
		leftClass, leftWeight := order.weight(left[index])
		rightClass, rightWeight := order.weight(right[index])
		if leftClass != rightClass {
			return leftClass - rightClass
		}
		if leftWeight != rightWeight {
			return leftWeight - rightWeight
		}
	}
	return len(left) - len(right)
}

// weight answers the class (space, punctuation, digit, letter) of a
// character and its rank within the class.
func (TextOrder) weight(character rune) (int, int) {
	punctuation := "_-,;:!?.'\"()[]{}@*/\\&#%`^+<=>|~$"
	switch {
	case unicode.IsSpace(character):
		return 0, int(character)
	case strings.ContainsRune(punctuation, character):
		return 1, strings.IndexRune(punctuation, character)
	case unicode.IsDigit(character):
		return 3, int(character)
	case unicode.IsLetter(character):
		return 4, int(character)
	}
	return 2, int(character)
}
