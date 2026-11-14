package devicebox

import "time"

type Limits struct{}

func (Limits) MaxDevices() int {
	return 200
}

func (Limits) MaxCiphertextBytes() int {
	return 256 * 1024
}

func (Limits) MaxWaiting() int {
	return 10000
}

func (Limits) Lifetime() time.Duration {
	return 30 * 24 * time.Hour
}

func (Limits) MaxAcknowledged() int {
	return 10
}

func (Limits) DefaultPage() int {
	return 50
}

func (Limits) MaxPage() int {
	return 100
}

func (Limits) PageCiphertextBytes() int {
	return 4 * 1024 * 1024
}

func (Limits) WriteBudget() int {
	return 25
}

func (Limits) TimeBudget() time.Duration {
	return 20 * time.Second
}

func (Limits) DirectoryTimeout() time.Duration {
	return 10 * time.Second
}
