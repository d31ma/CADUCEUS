type TotpRemove struct {
	CurrentPassword string `json:"currentPassword"`
	OneTimeCode     string `json:"oneTimeCode"`
}
