package main

type CodeComparison struct {
	Table ErrorsTable
	Used  []OwnerNumbers
}

func (comparison CodeComparison) Problems() []Problem {
	return append(comparison.codeProblems(), comparison.unusedProblems()...)
}

func (comparison CodeComparison) codeProblems() []Problem {
	problems := []Problem{}
	for _, owned := range comparison.Used {
		problems = append(problems, comparison.ownerProblems(owned)...)
	}
	return problems
}

func (comparison CodeComparison) ownerProblems(owned OwnerNumbers) []Problem {
	seen := map[int]UsedNumber{}
	problems := []Problem{}
	for _, used := range owned.Numbers {
		problems = append(problems, comparison.usedProblem(owned.Owner, used, seen)...)
		seen[used.Number] = comparison.first(seen, used)
	}
	return problems
}

func (CodeComparison) first(seen map[int]UsedNumber, used UsedNumber) UsedNumber {
	if earlier, isSeen := seen[used.Number]; isSeen {
		return earlier
	}
	return used
}

func (comparison CodeComparison) usedProblem(owner Owner, used UsedNumber, seen map[int]UsedNumber) []Problem {
	earlier, isSeen := seen[used.Number]
	switch {
	case !owner.Holds(used.Number):
		return []Problem{Problem{}.UsedOutsideBlock(used, owner)}
	case isSeen:
		return []Problem{Problem{}.UsedTwice(used, earlier)}
	case !comparison.Table.Lists(used.Number):
		return []Problem{Problem{}.NotListed(used)}
	}
	return nil
}

func (comparison CodeComparison) unusedProblems() []Problem {
	problems := []Problem{}
	for _, row := range comparison.Table.Rows {
		problems = append(problems, comparison.rowProblem(row, comparison.owned(row.App))...)
	}
	return problems
}

func (comparison CodeComparison) owned(app string) []OwnerNumbers {
	for _, owned := range comparison.Used {
		if owned.Owner.Name == app {
			return []OwnerNumbers{owned}
		}
	}
	return nil
}

func (CodeComparison) rowProblem(row ListedNumber, owned []OwnerNumbers) []Problem {
	switch {
	case len(owned) == 0:
		return nil
	case !owned[0].Owner.Holds(row.Number) || row.Number == owned[0].Owner.First || owned[0].Uses(row.Number):
		return nil
	}
	return []Problem{Problem{}.NotUsed(row)}
}
