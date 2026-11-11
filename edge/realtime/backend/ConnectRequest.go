package backend

type ConnectRequest struct {
	ConnectionId string `json:"connectionId"`
	Kind         string `json:"kind"`
}
