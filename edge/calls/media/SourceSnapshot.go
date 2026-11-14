package media

import "time"

type SourceSnapshot struct {
	DeviceId      string
	JoinedAt      time.Time
	LastSpokeAt   time.Time
	HasMicrophone bool
	CameraRids    []string
	ScreenSince   time.Time
}

func (source SourceSnapshot) HasCamera() bool {
	return len(source.CameraRids) > 0
}

func (source SourceSnapshot) SharesScreen() bool {
	return !source.ScreenSince.IsZero()
}
