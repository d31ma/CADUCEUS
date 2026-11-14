package realtime

import "time"

type SignalLimits struct{}

func (SignalLimits) MaximumTargets() int {
	return 200
}

func (SignalLimits) MaximumCiphertextBytes() int {
	return 65536
}

func (SignalLimits) MaximumInviteBytes() int {
	return 16384
}

func (SignalLimits) MaximumRingPeople() int {
	return 49
}

func (SignalLimits) MaximumCallPeople() int {
	return 50
}

func (SignalLimits) CallsPerMinute() int {
	return 10
}

func (SignalLimits) MaximumIdLength() int {
	return 64
}

func (SignalLimits) MinimumPairingIdLength() int {
	return 16
}

func (SignalLimits) PairingLifetime() time.Duration {
	return 2 * time.Minute
}

func (SignalLimits) DeviceRecheck() time.Duration {
	return 5 * time.Minute
}

func (SignalLimits) StaleCall() time.Duration {
	return 12 * time.Hour
}

func (SignalLimits) CallPushTimeout() time.Duration {
	return 2 * time.Second
}

func (SignalLimits) JoinTokenLifetime() time.Duration {
	return 60 * time.Second
}
