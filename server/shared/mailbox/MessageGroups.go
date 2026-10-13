// MessageGroups is the union-find the thread grouping joins messages with.
type MessageGroups struct {
	parent []int
}

// Of answers groups of size messages, each in a group of its own.
func (MessageGroups) Of(size int) *MessageGroups {
	parent := make([]int, size)
	for index := range parent {
		parent[index] = index
	}
	return &MessageGroups{parent: parent}
}

// Find answers the root of a message's group.
func (groups *MessageGroups) Find(index int) int {
	if groups.parent[index] == index {
		return index
	}
	groups.parent[index] = groups.Find(groups.parent[index])
	return groups.parent[index]
}

// Union joins the groups of two messages under the first one's root.
func (groups *MessageGroups) Union(left int, right int) {
	first, second := groups.Find(left), groups.Find(right)
	if first != second {
		groups.parent[second] = first
	}
}

// Connect joins a message to the one indexed under key, or indexes it there.
func (groups *MessageGroups) Connect(index map[string]int, key string, current int) {
	if existing, found := index[key]; found {
		groups.Union(current, existing)
		return
	}
	index[key] = current
}

// Grouped answers the emails of each group, in the order groups first appear.
func (groups *MessageGroups) Grouped(emails []map[string]any) [][]map[string]any {
	order := []int{}
	members := map[int][]map[string]any{}
	for index, email := range emails {
		root := groups.Find(index)
		if _, seen := members[root]; !seen {
			order = append(order, root)
		}
		members[root] = append(members[root], email)
	}
	grouped := make([][]map[string]any, 0, len(order))
	for _, root := range order {
		grouped = append(grouped, members[root])
	}
	return grouped
}
