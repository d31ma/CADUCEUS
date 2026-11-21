package scheduling

type workerSettings struct {
	Yon struct {
		Workers struct {
			Enabled *bool `json:"enabled"`
		} `json:"workers"`
	} `json:"yon"`
}
