package realtime

import (
	"slices"
	"time"
)

type CallStartBook struct {
	Storage Storage
	Now     func() time.Time
}

func (book CallStartBook) Allow(domainId string, deviceId string) error {
	record := CallStartRecord{}
	found, err := book.shelf().Find(domainId, map[string]any{"deviceId": deviceId}, &record)
	if err != nil {
		return err
	}
	recent := book.recent(record.StartedAt)
	err = book.under(recent)
	if err != nil {
		return err
	}
	record.StartedAt = append(recent, book.Now().UTC().Format(time.RFC3339Nano))
	return book.write(domainId, record, found, deviceId)
}

func (book CallStartBook) recent(startedAt []string) []string {
	since := book.Now().Add(-time.Minute)
	return slices.DeleteFunc(slices.Clone(startedAt), func(text string) bool {
		moment, err := time.Parse(time.RFC3339Nano, text)
		return err != nil || !moment.After(since)
	})
}

func (CallStartBook) under(recent []string) error {
	if len(recent) >= (SignalLimits{}).CallsPerMinute() {
		return CallRefusal{}.TooManyCalls()
	}
	return nil
}

func (book CallStartBook) write(domainId string, record CallStartRecord, found bool, deviceId string) error {
	if !found {
		return book.shelf().Insert(domainId, CallStartRecord{DeviceId: deviceId, StartedAt: record.StartedAt})
	}
	return book.shelf().Patch(domainId, record.ID, CallStartRecord{DeviceId: deviceId, StartedAt: record.StartedAt})
}

func (book CallStartBook) shelf() DomainShelf {
	return DomainShelf{Storage: book.Storage, Base: "callstarts"}
}
