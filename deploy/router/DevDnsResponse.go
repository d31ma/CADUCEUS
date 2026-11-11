package main

type DevDnsResponse struct {
	Status int            `json:"Status"`
	Answer []DevDnsAnswer `json:"Answer,omitempty"`
}
