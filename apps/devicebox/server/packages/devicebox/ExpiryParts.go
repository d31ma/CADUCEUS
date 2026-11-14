package devicebox

type ExpiryParts struct {
	Store   Store
	Counts  BoxCounts
	Staging EventStaging
	Desk    OutboxDesk
}
