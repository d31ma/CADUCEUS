type PasskeySetupRequest struct {
	CurrentPassword string `json:"currentPassword"`
	OneTimeCode     string `json:"oneTimeCode,omitempty"`
}
