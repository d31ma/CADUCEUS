// PresenceService tells who is online, per domain, from heartbeats stored in
// FYLO.
// @Service
type PresenceService struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Heartbeat marks a user as online now.
func (service *PresenceService) Heartbeat(email string, domains []string) error {
	return service.registry().Heartbeat(email, domains)
}

// SnapshotForDomains answers every recently seen user who shares at least
// one domain with the viewer.
func (service *PresenceService) SnapshotForDomains(viewerDomains []string) ([]map[string]any, error) {
	return service.registry().Snapshot(viewerDomains)
}

// IsOnline reports whether a user is online.
func (service *PresenceService) IsOnline(email string) (bool, error) {
	return service.registry().Online(email)
}

func (service *PresenceService) registry() PresenceRegistry {
	return PresenceRegistry{Store: CollectionStore{Database: service.fyloHelper.Database(), Collection: FyloCollections{}.Named().Presence}}
}
