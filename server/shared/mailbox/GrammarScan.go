// GrammarScan holds one check in progress.
type GrammarScan struct {
	text     string
	units    []int
	length   int
	disabled map[string]bool
	matches  []GrammarMatch
}
