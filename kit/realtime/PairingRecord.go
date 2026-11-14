package realtime

type PairingRecord struct {
	ID             string `json:"id,omitempty"`
	PairingId      string `json:"pairingId"`
	PersonId       string `json:"personId"`
	OpenerDeviceId string `json:"openerDeviceId"`
	JoinerDeviceId string `json:"joinerDeviceId,omitempty"`
	ExpiresAt      string `json:"expiresAt"`
}
