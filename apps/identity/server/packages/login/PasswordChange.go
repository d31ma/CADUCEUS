package login

type PasswordChange struct {
	CurrentPassword string
	NewPassword     string
	OneTimeCode     string
	Kept            bool
}
