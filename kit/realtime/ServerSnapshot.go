package realtime

type ServerSnapshot struct {
	Yon struct {
		Server struct {
			Port int `json:"port"`
		} `json:"server"`
	} `json:"yon"`
}
