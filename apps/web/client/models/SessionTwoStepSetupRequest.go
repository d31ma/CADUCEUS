type SessionTwoStepSetupRequest struct {
	CurrentPassword string `json:"currentPassword"`
	OneTimeCode     string `json:"oneTimeCode,omitempty"`
}
