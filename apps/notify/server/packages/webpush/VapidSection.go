package webpush

type VapidSection struct {
	Public  KeyValue `json:"public"`
	Private KeyValue `json:"private"`
	Subject string   `json:"subject"`
}
