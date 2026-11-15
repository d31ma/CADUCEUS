package backups

type Limits struct{}

func (Limits) MostKeySlots() int {
	return 10
}

func (Limits) LargestKeySlot() int {
	return 1024
}

func (Limits) RecoveryRounds() int64 {
	return 600000
}

func (Limits) RecoveryKind() string {
	return "recovery"
}

func (Limits) PasskeyKind() string {
	return "passkey"
}

func (Limits) MediaType() string {
	return "application/octet-stream"
}

func (Limits) FileName() string {
	return "mesail-backup.bin"
}
