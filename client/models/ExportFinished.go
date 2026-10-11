// How an export ended. Canceled and Failure are mutually exclusive.
type ExportFinished struct {
	Messages         int    `json:"messages"`
	SourceBytesLabel string `json:"sourceBytesLabel"`
	Canceled         bool   `json:"canceled"`
	Failure          string `json:"failure"`
}
