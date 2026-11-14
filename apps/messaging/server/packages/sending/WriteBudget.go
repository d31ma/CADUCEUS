package sending

import "mesail.dev/packages/core"

type WriteBudget struct{}

func (budget WriteBudget) planned(draft Draft) int {
	lists := Lists{}
	return budget.fixedWrites() + len(draft.Placement.Newcomers) + lists.oneIf(draft.IsEmail) + lists.oneIf(draft.Thread.RootId != "") + len(draft.Request.AttachmentIds)
}

func (budget WriteBudget) check(planned int) error {
	if planned <= budget.limit() {
		return nil
	}
	return core.InvalidRequest{}.TooManyWrites()
}

func (budget WriteBudget) remaining(planned int) int {
	return max(0, budget.limit()-planned)
}

func (WriteBudget) limit() int {
	return 25
}

func (WriteBudget) fixedWrites() int {
	return 4
}
