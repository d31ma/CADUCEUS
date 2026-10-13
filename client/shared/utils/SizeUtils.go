import "strconv"

// SizeUtils writes byte counts as B, KB or MB.
type SizeUtils struct{}

// Label is "512 B", "3.4 KB", "12 KB" or "1.5 MB".
func (SizeUtils) Label(bytes int) string {
	return SizeUtils{}.label(float64(max(0, bytes)), SizeUtils{}.kilobyte())
}

func (SizeUtils) label(bytes float64, kilobyte float64) string {
	switch {
	case bytes < kilobyte:
		return SizeUtils{}.number(bytes, 0) + " B"
	case bytes < kilobyte*kilobyte:
		return SizeUtils{}.number(bytes/kilobyte, map[bool]int{true: 1}[bytes < SizeUtils{}.precise()*kilobyte]) + " KB"
	}
	return SizeUtils{}.number(bytes/kilobyte/kilobyte, 1) + " MB"
}

func (SizeUtils) number(value float64, digits int) string {
	return strconv.FormatFloat(value, 'f', digits, 64)
}

func (SizeUtils) kilobyte() float64 { return 1024 }

// precise is the size, in kilobytes, below which a label keeps one decimal.
func (SizeUtils) precise() float64 { return 10 }
