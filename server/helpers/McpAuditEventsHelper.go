import (
	"math"
	"sort"
	"strings"
)

// McpAuditEventsHelper keeps the audit trail of MCP tool calls, newest first,
// at most McpAuditEventsRecords.RetainedEvents per mailbox.
// @Helper
type McpAuditEventsHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// Put stores one event and deletes the owner's events beyond the retained
// number.
func (helper *McpAuditEventsHelper) Put(event map[string]any) error {
	if _, err := helper.store().Put(event); err != nil {
		return err
	}
	events, err := helper.ListForOwner(RecordValues{}.Text(event[helper.ownerField()]), math.MaxInt)
	if err != nil {
		return err
	}
	retained := McpAuditEventsRecords{}.RetainedEvents()
	if len(events) <= retained {
		return nil
	}
	return helper.deleteAll(events[retained:])
}

// ListForOwner answers the owner's newest events, at most limit, as
// {docId, ...event, parameterKeys}.
func (helper *McpAuditEventsHelper) ListForOwner(ownerEmail string, limit int) ([]map[string]any, error) {
	documents, err := helper.store().Equal(helper.ownerField(), strings.ToLower(ownerEmail))
	if err != nil {
		return nil, err
	}
	records := McpAuditEventsRecords{}
	events := StoredRecords{}.WithDocumentIds(documents)
	for _, event := range events {
		event["parameterKeys"] = records.ParameterKeys(event["parameterKeysJson"])
	}
	sort.SliceStable(events, func(left, right int) bool {
		return TextOrder{}.Compare(records.CreatedAtText(events[right]), records.CreatedAtText(events[left])) < 0
	})
	if len(events) > max(0, limit) {
		events = events[:max(0, limit)]
	}
	return events, nil
}

// ClearForOwner deletes every event of the owner and answers how many.
func (helper *McpAuditEventsHelper) ClearForOwner(ownerEmail string) (int, error) {
	events, err := helper.ListForOwner(ownerEmail, math.MaxInt)
	if err != nil {
		return 0, err
	}
	return len(events), helper.deleteAll(events)
}

func (helper *McpAuditEventsHelper) deleteAll(events []map[string]any) error {
	for _, event := range events {
		if err := helper.store().Delete(RecordValues{}.Text(event["docId"])); err != nil {
			return err
		}
	}
	return nil
}

func (McpAuditEventsHelper) ownerField() string { return "ownerEmail" }

func (helper *McpAuditEventsHelper) store() CollectionStore {
	return CollectionStore{Database: helper.fyloHelper.Database(), Collection: FyloCollections{}.Named().McpAuditEvents}
}
