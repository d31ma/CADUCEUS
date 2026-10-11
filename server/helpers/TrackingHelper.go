import "sync"

// TrackingHelper records verified opens and clicks on the Sent copy of a
// message and builds the owner's activity feed from stored mail.
// @Helper
type TrackingHelper struct {
	// @Needs
	fyloHelper *FyloHelper
	// pending runs one record at a time, so two events of one message
	// never overwrite each other's counts.
	pending sync.Mutex
}

// RecordOpen records one recipient-level open of the message with the
// tracking id, after the route verified the tracking capability, and reports
// whether it was recorded.
func (helper *TrackingHelper) RecordOpen(trackingId string, recipient string) (bool, error) {
	return helper.record(trackingId, func(email map[string]any) map[string]any {
		return TrackingEvents{}.Opened(email, recipient, EpochTime{}.NowText())
	})
}

// RecordClick records one recipient-level click when the target exactly
// matches the link stored with the Sent copy, and reports whether it was
// recorded.
func (helper *TrackingHelper) RecordClick(click TrackingClick) (bool, error) {
	return helper.record(click.TrackingId, func(email map[string]any) map[string]any {
		return TrackingEvents{}.Clicked(email, click, EpochTime{}.NowText())
	})
}

// record patches the message with the tracking id, one record at a time.
func (helper *TrackingHelper) record(trackingId string, patchOf func(map[string]any) map[string]any) (bool, error) {
	id := TrackingRecords{}.NormalizeTrackingId(trackingId)
	if id == "" {
		return false, nil
	}
	helper.pending.Lock()
	defer helper.pending.Unlock()
	documentId, email, err := helper.findByTrackingId(id)
	if err != nil || documentId == "" {
		return false, err
	}
	patch := patchOf(email)
	if patch == nil {
		return false, nil
	}
	return true, helper.emails().Patch(documentId, patch)
}

// Activity answers the exact-owner activity feed and analytics dashboard.
func (helper *TrackingHelper) Activity(options TrackingActivityOptions) (map[string]any, error) {
	documents, err := helper.emails().All()
	if err != nil {
		return nil, err
	}
	return TrackingActivity{Options: options, Now: EpochTime{}.Now()}.Built(StoredRecords{}.Listed(documents)), nil
}

// findByTrackingId answers the document id and stored email with the
// tracking id, or "" and nil.
func (helper *TrackingHelper) findByTrackingId(trackingId string) (string, map[string]any, error) {
	id := TrackingRecords{}.NormalizeTrackingId(trackingId)
	if id == "" {
		return "", nil, nil
	}
	record, err := helper.emails().First("trackingId", id)
	return record.Id, record.Data, err
}

func (helper *TrackingHelper) emails() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().Emails}
}
