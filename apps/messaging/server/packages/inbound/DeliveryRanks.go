package inbound

import "mesail.dev/packages/core"

type DeliveryRanks struct{}

func (ranks DeliveryRanks) Allows(oldStatus string, newStatus string) bool {
	table := ranks.table()
	newRank, known := table[newStatus]
	return known && newRank >= table[oldStatus]
}

func (DeliveryRanks) table() map[string]int {
	statuses := core.DeliveryStatuses{}
	return map[string]int{
		statuses.Queued():   0,
		statuses.Deferred(): 1,
		statuses.Sent():     2,
		statuses.Bounced():  2,
		statuses.Failed():   2,
	}
}
