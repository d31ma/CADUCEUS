import "time"

type WriteBudget struct {
	Left     int
	Used     int
	Deadline time.Time
	Short    bool
}

func (budget *WriteBudget) Take(writes int) bool {
	allowed := budget.allows(writes)
	budget.spend(allowed, writes)
	return allowed
}

func (budget *WriteBudget) Spend(writes int, step func() error) error {
	return budget.stepWhen(budget.Take(writes), step)
}

func (budget *WriteBudget) Report() RemovalReport {
	return RemovalReport{Writes: budget.Used, More: budget.Short}
}

func (budget *WriteBudget) stepWhen(allowed bool, step func() error) error {
	if !allowed {
		return nil
	}
	return step()
}

func (budget *WriteBudget) allows(writes int) bool {
	return budget.Left >= writes && time.Now().Before(budget.Deadline)
}

func (budget *WriteBudget) spend(allowed bool, writes int) {
	if !allowed {
		budget.Short = true
		return
	}
	budget.Left -= writes
	budget.Used += writes
}
