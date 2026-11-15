package storage

import (
	"cmp"
	"slices"
	"time"

	"mesail.dev/packages/core"
)

type ChangeLogs struct {
	Documents Documents
	Source    DataSource
	Names     CollectionNames
}

func (logs ChangeLogs) Register(collection string, month string) error {
	exists, err := logs.Documents.Exists(collection)
	if err != nil {
		return err
	}
	return logs.recordUnless(exists, ChangeLogRecord{Name: collection, Month: month})
}

func (logs ChangeLogs) Cleanup(now time.Time) (CleanupReport, error) {
	return logs.DropBefore(core.ChangeRetention{}.FirstKeptMonth(now), logs.mostPerCall())
}

func (logs ChangeLogs) DropBefore(month string, most int) (CleanupReport, error) {
	records, err := logs.records()
	if err != nil {
		return CleanupReport{}, err
	}
	old := logs.older(records, month)
	dropped, err := logs.dropped(old[:min(len(old), most)])
	return CleanupReport{Dropped: dropped, More: len(old) > most}, err
}

func (logs ChangeLogs) recordUnless(exists bool, record ChangeLogRecord) error {
	if exists {
		return nil
	}
	known, err := logs.named(record.Name)
	return cmp.Or(err, logs.insertUnless(len(known) > 0 || err != nil, record))
}

func (logs ChangeLogs) insertUnless(skip bool, record ChangeLogRecord) error {
	if skip {
		return nil
	}
	_, err := logs.Documents.Insert(logs.Names.ChangeLogs(), record)
	return err
}

func (logs ChangeLogs) named(name string) ([]ChangeLogRecord, error) {
	return logs.matching(map[string]any{logs.nameField(): name})
}

func (logs ChangeLogs) records() ([]ChangeLogRecord, error) {
	return logs.matching(map[string]any{})
}

func (logs ChangeLogs) matching(where map[string]any) ([]ChangeLogRecord, error) {
	records := []ChangeLogRecord{}
	err := logs.Documents.Ensure(logs.Names.ChangeLogs())
	if err != nil {
		return records, err
	}
	return records, logs.Documents.All(logs.Names.ChangeLogs(), where, &records)
}

func (ChangeLogs) older(records []ChangeLogRecord, month string) []ChangeLogRecord {
	return slices.DeleteFunc(slices.Clone(records), func(record ChangeLogRecord) bool {
		return record.Month >= month
	})
}

func (logs ChangeLogs) dropped(records []ChangeLogRecord) (int, error) {
	count := 0
	for _, record := range records {
		err := logs.drop(record)
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (logs ChangeLogs) drop(record ChangeLogRecord) error {
	logs.Documents.Known.Forget(record.Name)
	_, err := logs.Source.DropCollection(record.Name)
	if err != nil {
		return logs.Documents.unavailable(err)
	}
	return logs.Documents.Delete(logs.Names.ChangeLogs(), record.Id)
}

func (ChangeLogs) mostPerCall() int {
	return 12
}

func (ChangeLogs) nameField() string {
	return "name"
}
