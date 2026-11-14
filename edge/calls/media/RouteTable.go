package media

type RouteTable struct {
	outputs map[FeedKey][]*SlotOutput
}

func (RouteTable) From(members []*Member) *RouteTable {
	table := &RouteTable{outputs: map[FeedKey][]*SlotOutput{}}
	table.addMembers(members)
	return table
}

func (table *RouteTable) Outputs(feed FeedKey) []*SlotOutput {
	return table.outputs[feed]
}

func (table *RouteTable) addMembers(members []*Member) {
	for _, member := range members {
		table.addSlots(member.Slots())
	}
}

func (table *RouteTable) addSlots(slots []*SlotOutput) {
	for _, slot := range slots {
		table.addFeeds(slot, slot.Feeds())
	}
}

func (table *RouteTable) addFeeds(slot *SlotOutput, feeds []FeedKey) {
	for _, feed := range feeds {
		table.outputs[feed] = append(table.outputs[feed], slot)
	}
}
