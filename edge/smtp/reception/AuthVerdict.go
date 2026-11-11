package reception

type AuthVerdict struct {
	Results     AuthResults
	Reject      bool
	HeaderField string
}
