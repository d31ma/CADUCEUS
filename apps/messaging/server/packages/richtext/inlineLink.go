package richtext

type inlineLink struct {
	label   string
	address string
	end     int
	found   bool
}
