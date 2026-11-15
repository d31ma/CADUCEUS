type PasskeyRemove struct {
	CurrentPassword string `json:"currentPassword"`
	OneTimeCode     string `json:"oneTimeCode,omitempty"`
	KeepSignedIn    bool   `json:"keepSignedIn,omitempty"`
}
