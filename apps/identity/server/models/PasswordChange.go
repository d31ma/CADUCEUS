type PasswordChange struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
	OneTimeCode     string `json:"oneTimeCode,omitempty"`
	SessionKey      string `json:"sessionKey"`
}
