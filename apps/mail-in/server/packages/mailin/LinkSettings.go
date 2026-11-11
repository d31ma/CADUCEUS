package mailin

type LinkSettings struct {
	Base    string `json:"base"`
	Seconds int64  `json:"seconds"`
	Secret  string `json:"secret"`
}
