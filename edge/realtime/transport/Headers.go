package transport

type Headers struct{}

func (Headers) Secret() string      { return "X-Realtime-Gateway" }
func (Headers) Origin() string      { return "Origin" }
func (Headers) ContentType() string { return "Content-Type" }
func (Headers) Json() string        { return "application/json" }
