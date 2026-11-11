package connections

import "time"

type SessionSettings struct {
	AuthWindow        time.Duration
	PingInterval      time.Duration
	KeepAliveInterval time.Duration
	WriteTimeout      time.Duration
	QueueLimit        int
}

func DefaultSessionSettings() SessionSettings {
	return SessionSettings{
		AuthWindow:        10 * time.Second,
		PingInterval:      30 * time.Second,
		KeepAliveInterval: 25 * time.Second,
		WriteTimeout:      10 * time.Second,
		QueueLimit:        64,
	}
}
