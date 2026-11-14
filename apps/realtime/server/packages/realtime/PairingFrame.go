package realtime

type PairingFrame struct {
	Type         string `json:"type"`
	RequestId    string `json:"requestId,omitempty"`
	PairingId    string `json:"pairingId"`
	PeerDeviceId string `json:"peerDeviceId,omitempty"`
	ExpiresAt    string `json:"expiresAt,omitempty"`
}
