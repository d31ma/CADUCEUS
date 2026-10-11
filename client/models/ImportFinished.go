// How an import ended.
type ImportFinished struct {
	Imported     int    `json:"imported"`
	Failures     int    `json:"failures"`
	FirstFailure string `json:"firstFailure"`
	Canceled     bool   `json:"canceled"`
	Failure      string `json:"failure"`
}
