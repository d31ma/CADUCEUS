type SessionTwoStepRemove struct {
	CurrentPassword string `json:"currentPassword"`
	OneTimeCode     string `json:"oneTimeCode"`
}
