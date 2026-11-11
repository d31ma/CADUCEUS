package realtime

type DeskParts struct {
	Connections ConnectionBook
	FanOut      FanOut
	Presence    PresenceKeeper
}
