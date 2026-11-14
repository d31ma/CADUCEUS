package media

import "slices"

type SlotAssignment struct{}

func (assignment SlotAssignment) Assign(previous []string, wanted []string) []string {
	kept := assignment.kept(previous, wanted)
	return assignment.filled(kept, assignment.missing(kept, wanted))
}

func (assignment SlotAssignment) kept(previous []string, wanted []string) []string {
	kept := make([]string, len(previous))
	for index, deviceId := range previous {
		kept[index] = assignment.stillWanted(deviceId, wanted)
	}
	return kept
}

func (SlotAssignment) stillWanted(deviceId string, wanted []string) string {
	if !slices.Contains(wanted, deviceId) {
		return ""
	}
	return deviceId
}

func (SlotAssignment) missing(kept []string, wanted []string) []string {
	return slices.DeleteFunc(slices.Clone(wanted), func(deviceId string) bool {
		return slices.Contains(kept, deviceId)
	})
}

func (assignment SlotAssignment) filled(kept []string, queue []string) []string {
	for index := range kept {
		kept[index], queue = assignment.place(kept[index], queue)
	}
	return kept
}

func (SlotAssignment) place(current string, queue []string) (string, []string) {
	if current != "" || len(queue) == 0 {
		return current, queue
	}
	return queue[0], queue[1:]
}
