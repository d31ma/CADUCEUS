type DomainCheck struct {
	State string   `json:"state"`
	Dns   DnsState `json:"dns"`
}
