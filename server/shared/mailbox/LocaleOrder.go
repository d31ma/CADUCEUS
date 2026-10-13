import (
	"strings"
	"unicode"
)

// LocaleOrder approximates the root-locale order of String.localeCompare:
// letters compare without case first (white space and punctuation before
// digits, digits before letters), then lower case before upper case, then
// code points. Accents are not folded.
type LocaleOrder struct{}

// Compare answers a negative number, zero or a positive number as left
// sorts before, with or after right.
func (order LocaleOrder) Compare(left string, right string) int {
	if primary := order.compareKeys(order.primary(left), order.primary(right)); primary != 0 {
		return primary
	}
	if tertiary := order.compareKeys(order.tertiary(left), order.tertiary(right)); tertiary != 0 {
		return tertiary
	}
	return strings.Compare(left, right)
}

func (LocaleOrder) primary(value string) []int {
	keys := []int{}
	for _, letter := range value {
		lower := unicode.ToLower(letter)
		switch {
		case unicode.IsLetter(lower):
			keys = append(keys, 3<<21|int(lower))
		case unicode.IsDigit(lower):
			keys = append(keys, 2<<21|int(lower))
		default:
			keys = append(keys, 1<<21|int(lower))
		}
	}
	return keys
}

func (LocaleOrder) tertiary(value string) []int {
	keys := []int{}
	for _, letter := range value {
		keys = append(keys, map[bool]int{true: 1, false: 0}[unicode.IsUpper(letter)])
	}
	return keys
}

func (LocaleOrder) compareKeys(left []int, right []int) int {
	for index := 0; index < len(left) && index < len(right); index++ {
		if left[index] != right[index] {
			return left[index] - right[index]
		}
	}
	return len(left) - len(right)
}
