package devicebox

import "maps"

type BoxCounts struct {
	documents Documents
}

func NewBoxCounts(documents Documents) BoxCounts {
	return BoxCounts{documents: documents}
}

func (counts BoxCounts) Room(domainId string, deviceKeys []string) error {
	buckets, err := counts.read(domainId, counts.bucketsOf(deviceKeys))
	if err != nil {
		return err
	}
	return counts.fullRefusal(buckets, deviceKeys)
}

func (counts BoxCounts) Add(domainId string, deltas map[string]int) (int, error) {
	buckets, err := counts.read(domainId, counts.bucketsOf(counts.keysOf(deltas)))
	if err != nil {
		return 0, err
	}
	writes := 0
	for name, bucket := range buckets {
		err = counts.save(domainId, counts.applied(bucket, name, deltas))
		writes++
		if err != nil {
			return writes, err
		}
	}
	return writes, nil
}

func (counts BoxCounts) Buckets(deviceKeys []string) int {
	return len(counts.bucketsOf(deviceKeys))
}

func (counts BoxCounts) read(domainId string, names map[string]bool) (map[string]BoxCount, error) {
	buckets := map[string]BoxCount{}
	for name := range names {
		bucket, err := counts.bucket(domainId, name)
		if err != nil {
			return nil, err
		}
		buckets[name] = bucket
	}
	return buckets, nil
}

func (counts BoxCounts) bucket(domainId string, name string) (BoxCount, error) {
	statement, err := SqlText{}.Bucket(Names{}.Counts(domainId), name)
	if err != nil {
		return BoxCount{}, err
	}
	found := []BoxCount{}
	err = counts.documents.Rows(statement, &found)
	return counts.firstOrNew(found, name), err
}

func (BoxCounts) firstOrNew(found []BoxCount, name string) BoxCount {
	if len(found) == 0 {
		return BoxCount{Bucket: name, Counts: map[string]int{}}
	}
	return found[0]
}

func (counts BoxCounts) applied(bucket BoxCount, name string, deltas map[string]int) BoxCount {
	updated := maps.Clone(bucket.Counts)
	for deviceKey, delta := range deltas {
		counts.applyOne(updated, name, deviceKey, delta)
	}
	bucket.Counts = updated
	return bucket
}

func (BoxCounts) applyOne(updated map[string]int, name string, deviceKey string, delta int) {
	if (DeviceKey{}).Bucket(deviceKey) != name {
		return
	}
	updated[deviceKey] = max(0, updated[deviceKey]+delta)
}

func (counts BoxCounts) save(domainId string, bucket BoxCount) error {
	if bucket.Id == "" {
		_, err := counts.documents.Insert(Names{}.Counts(domainId), bucket)
		return err
	}
	return counts.documents.Patch(Names{}.Counts(domainId), bucket.Id, countsPatch{Counts: bucket.Counts})
}

func (BoxCounts) fullRefusal(buckets map[string]BoxCount, deviceKeys []string) error {
	for _, deviceKey := range deviceKeys {
		if buckets[(DeviceKey{}).Bucket(deviceKey)].Counts[deviceKey] >= (Limits{}).MaxWaiting() {
			return BoxFull{}.Device()
		}
	}
	return nil
}

func (BoxCounts) bucketsOf(deviceKeys []string) map[string]bool {
	names := map[string]bool{}
	for _, deviceKey := range deviceKeys {
		names[DeviceKey{}.Bucket(deviceKey)] = true
	}
	return names
}

func (BoxCounts) keysOf(deltas map[string]int) []string {
	keys := []string{}
	for deviceKey := range deltas {
		keys = append(keys, deviceKey)
	}
	return keys
}
