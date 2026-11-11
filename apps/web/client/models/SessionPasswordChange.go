type SessionPasswordChange struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	OneTimeCode     string `json:"oneTimeCode,omitempty"`
}
